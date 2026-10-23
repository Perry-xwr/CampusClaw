from __future__ import annotations

import json
import subprocess
import shlex
import urllib.error
import urllib.request

from common import check_status


def _upload(ctx, filename: str, title: str, content: bytes):
    boundary = "CampusClaw" + ctx.run_id.replace("-", "")
    body = (
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"title\"\r\n\r\n{title}\r\n"
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"{filename}\"\r\n"
        "Content-Type: text/markdown; charset=utf-8\r\n\r\n"
    ).encode("utf-8") + content + f"\r\n--{boundary}--\r\n".encode("ascii")
    request = urllib.request.Request(
        ctx.base_url + "/api/materials",
        data=body,
        headers={"Authorization": f"Bearer {ctx.token_a_teacher}", "Content-Type": f"multipart/form-data; boundary={boundary}"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        return response.status, json.loads(response.read().decode("utf-8"))


def _sse(ctx, session_id: int, question: str):
    payload = json.dumps({"content": question}, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        f"{ctx.base_url}/api/sessions/{session_id}/messages",
        data=payload,
        headers={"Authorization": f"Bearer {ctx.token_a_student}", "Accept": "text/event-stream", "Content-Type": "application/json"},
        method="POST",
    )
    events = {}
    current = ""
    with urllib.request.urlopen(request, timeout=30) as response:
        check_status(response.status, 200)
        assert "text/event-stream" in response.headers.get("Content-Type", ""), response.headers
        for raw in response:
            line = raw.decode("utf-8").rstrip("\r\n")
            if line.startswith("event:"):
                current = line[6:].strip()
            elif line.startswith("data:") and current:
                events.setdefault(current, []).append(json.loads(line[5:].strip()))
    return events


def _qdrant_material_points(material_id: int) -> list:
    payload = json.dumps({"limit": 10, "filter": {"must": [
        {"key": "material_id", "match": {"value": material_id}},
        {"key": "class_id", "match": {"value": 1}},
    ]}, "with_payload": False, "with_vector": False}, separators=(",", ":"))
    shell = "wget -qO- --header='Content-Type: application/json' --post-data='" + payload + "' http://qdrant:6333/collections/campusclaw_chunks/points/scroll"
    result = subprocess.run(["docker", "compose", "exec", "-T", "api", "sh", "-lc", shell], cwd=__import__("common").ROOT, capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=20)
    if result.returncode:
        raise AssertionError("Qdrant consistency query failed")
    return json.loads(result.stdout).get("result", {}).get("points", [])


def _assert_upload_absent(stored_filename: str):
    assert stored_filename.replace(".", "").replace("-", "").replace("_", "").isalnum(), "unexpected stored filename"
    result = subprocess.run(
        ["docker", "compose", "exec", "-T", "api", "sh", "-lc", "test ! -f " + shlex.quote("/data/uploads/" + stored_filename)],
        cwd=__import__("common").ROOT,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        timeout=20,
    )
    if result.returncode:
        raise AssertionError("deleted material file remained in the uploads volume")


def run(ctx):
    assert ctx.prompt_id and ctx.skill_id, "run prompt and skill tests first"
    seed_a_status, _, seed_a_body = ctx.request("GET", "/api/materials/1/file", ctx.token_a_teacher)
    check_status(seed_a_status, 200, seed_a_body[:100])
    assert "钙钛矿".encode("utf-8") in seed_a_body, "A-class bundled seed file was not restored with its source text"
    seed_b_status, _, seed_b_body = ctx.request("GET", "/api/materials/2/file", ctx.token_b_student)
    check_status(seed_b_status, 200, seed_b_body[:100])
    assert "量子密钥分发".encode("utf-8") in seed_b_body, "B-class bundled seed file was not restored with its source text"
    isolated_status, _, _ = ctx.request("GET", "/api/materials/2/file", ctx.token_a_student)
    check_status(isolated_status, 404)

    title = "APITEST-DELETE-" + ctx.run_id
    filename = "apitest-delete-" + ctx.run_id + ".md"
    marker = "temporary-material-marker-" + ctx.run_id
    content = ("# 临时删除集成测试\n\n此资料用于验证本轮自动化删除，唯一检索标记为 " + marker + "。删除后该标记不能再从检索中返回。\n") .encode("utf-8")
    status, uploaded = _upload(ctx, filename, title, content)
    check_status(status, 201, uploaded)
    material_id = int(uploaded["id"])
    ctx.created_materials.append((material_id, title))
    assert uploaded.get("index_status") == "ready", f"temporary material indexing failed: {uploaded!r}"

    database = ctx.db_scalar(f"SELECT CONCAT(m.class_id, ':', m.uploader_id, ':', m.deletion_pending, ':', (SELECT COUNT(*) FROM knowledge_entries k WHERE k.material_id=m.id), ':', (SELECT COUNT(*) FROM knowledge_chunks c WHERE c.material_id=m.id AND c.index_status='ready')) FROM materials m WHERE m.id={material_id} AND m.title='{title}'")
    assert database == "1:1:0:1:1", f"uploaded material/database state mismatch: {database!r}"
    stored_filename = ctx.db_scalar(f"SELECT stored_filename FROM materials WHERE id={material_id} AND class_id=1 AND uploader_id=1 AND title='{title}'")

    download_status, _, downloaded = ctx.request("GET", f"/api/materials/{material_id}/file", ctx.token_a_teacher)
    check_status(download_status, 200, downloaded[:200])
    assert downloaded == content, "teacher download did not return the uploaded bytes"

    denied, body = ctx.json("DELETE", f"/api/materials/{material_id}", ctx.token_a_student)
    check_status(denied, 403, body)
    foreign, body = ctx.json("DELETE", "/api/materials/2", ctx.token_a_teacher)
    check_status(foreign, 404, body)
    assert ctx.db_scalar("SELECT COUNT(*) FROM materials WHERE id=2 AND class_id=2") == "1", "cross-class delete changed the foreign material"

    session_status, session = ctx.json("POST", "/api/sessions", ctx.token_a_student, {"title": title, "prompt_id": ctx.prompt_id, "skill_id": ctx.skill_id})
    check_status(session_status, 201, session)
    session_id = int(session["id"])
    ctx.created_sessions.append(session_id)
    events = _sse(ctx, session_id, marker)
    citations = events.get("sources", [{}])[0].get("citations", [])
    assert any(int(citation["material_id"]) == material_id for citation in citations), f"temporary retrieval citation missing: {citations!r}"
    done = events.get("done", [{}])[0]
    assert done.get("message_id"), f"SSE did not complete: {events!r}"
    persisted_citation = ctx.db_scalar(f"SELECT JSON_CONTAINS(citations, JSON_OBJECT('material_id', {material_id})) FROM chat_messages WHERE id={int(done['message_id'])} AND session_id={session_id} AND role='assistant'")
    assert persisted_citation == "1", f"temporary material citation was not persisted before deletion: {persisted_citation!r}"
    citation_hash = ctx.db_scalar(f"SELECT SHA2(CAST(citations AS CHAR), 256) FROM chat_messages WHERE id={int(done['message_id'])} AND session_id={session_id} AND role='assistant'")
    assert citation_hash and len(citation_hash) == 64, "persisted citation record is missing or invalid"

    qdrant_before = _qdrant_material_points(material_id)
    assert len(qdrant_before) == 1, f"expected one Qdrant point before deletion, got {len(qdrant_before)}"
    ctx.db_exec(f"UPDATE materials SET deletion_pending=TRUE WHERE id={material_id} AND class_id=1 AND uploader_id=1 AND title='{title}'")
    pending_teacher_status, pending_teacher = ctx.json("GET", "/api/materials", ctx.token_a_teacher)
    check_status(pending_teacher_status, 200, pending_teacher)
    pending_rows = [item for item in pending_teacher["materials"] if int(item["id"]) == material_id]
    assert len(pending_rows) == 1 and pending_rows[0].get("deletion_pending") is True, "teacher cannot see retryable pending material"
    pending_student_status, pending_student = ctx.json("GET", "/api/materials", ctx.token_a_student)
    check_status(pending_student_status, 200, pending_student)
    assert all(int(item["id"]) != material_id for item in pending_student["materials"]), "student can see a pending deletion record"
    pending_search_status, pending_search = ctx.json("POST", "/api/knowledge/search", ctx.token_a_student, {"query": marker, "mode": "hybrid"})
    check_status(pending_search_status, 200, pending_search)
    assert all(int(hit["material_id"]) != material_id for hit in pending_search["hits"]), "pending material remained searchable before retry"
    removed, _, _ = ctx.request("DELETE", f"/api/materials/{material_id}", ctx.token_a_teacher)
    check_status(removed, 204)
    ctx.created_materials.remove((material_id, title))
    _assert_upload_absent(stored_filename)

    remaining = ctx.db_scalar(f"SELECT (SELECT COUNT(*) FROM materials WHERE id={material_id}) + (SELECT COUNT(*) FROM knowledge_entries WHERE material_id={material_id}) + (SELECT COUNT(*) FROM knowledge_chunks WHERE material_id={material_id})")
    assert remaining == "0", f"material or RAG rows remained after delete: {remaining}"
    assert _qdrant_material_points(material_id) == [], "Qdrant retained a vector for the deleted material"
    missing_file, _, _ = ctx.request("GET", f"/api/materials/{material_id}/file", ctx.token_a_teacher)
    check_status(missing_file, 404)
    no_hit, search = ctx.json("POST", "/api/knowledge/search", ctx.token_a_student, {"query": marker, "mode": "hybrid"})
    check_status(no_hit, 200, search)
    assert not any(int(hit["material_id"]) == material_id for hit in search["hits"]), "deleted material remained searchable"
    history_hash = ctx.db_scalar(f"SELECT SHA2(CAST(citations AS CHAR), 256) FROM chat_messages WHERE id={int(done['message_id'])} AND session_id={session_id} AND role='assistant'")
    assert history_hash == citation_hash, "deleting material changed historical message citation data"
    return "临时资料上传/下载、学生拒绝、跨班拒绝、SSE 引用、DB/Qdrant/文件删除、RAG 无命中及历史引用保留均通过"
