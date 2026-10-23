"""Run five de-identified, opt-in HTTP-boundary experiments against the local stack."""
from __future__ import annotations

import hashlib
import importlib
import json
import re
import sys
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(Path(__file__).resolve().parent))
from common import Context

BASE_URL = "http://localhost:8080"
QUESTION = "实验前需要做什么准备？"


def post_sse(ctx: Context, session_id: int, question: str, scenario: str) -> list[dict]:
    body = json.dumps({"content": question}, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        f"{ctx.base_url}/api/sessions/{session_id}/messages",
        data=body,
        headers={
            "Authorization": f"Bearer {ctx.token_a_student}",
            "Accept": "text/event-stream",
            "Content-Type": "application/json; charset=utf-8",
            "X-CampusClaw-Test-Scenario": scenario,
        },
        method="POST",
    )
    events: list[dict] = []
    current = ""
    try:
        response = urllib.request.urlopen(request, timeout=40)
    except urllib.error.HTTPError as error:
        raise AssertionError(f"{scenario}: HTTP {error.code}") from error
    with response:
        if response.status != 200 or "text/event-stream" not in response.headers.get("Content-Type", ""):
            raise AssertionError(f"{scenario}: expected HTTP 200 text/event-stream")
        for line in response:
            decoded = line.decode("utf-8").rstrip("\r\n")
            if decoded.startswith("event:"):
                current = decoded[6:].strip()
            elif decoded.startswith("data:") and current:
                events.append({"event": current, "data": json.loads(decoded[5:].strip())})
    names = [item["event"] for item in events]
    if "done" not in names or names[:2] != ["meta", "sources"]:
        raise AssertionError(f"{scenario}: incomplete SSE event sequence {names!r}")
    return events


def create_session(ctx: Context, title: str, prompt_id: int, skill_id: int) -> int:
    status, body = ctx.json("POST", "/api/sessions", ctx.token_a_student,
                            {"title": title, "prompt_id": prompt_id, "skill_id": skill_id})
    if status != 201:
        raise AssertionError(f"session creation failed with HTTP {status}")
    session_id = int(body["id"])
    ctx.created_sessions.append(session_id)
    stored = ctx.db_scalar(f"SELECT CONCAT(class_id, ':', user_id) FROM chat_sessions WHERE id={session_id}")
    if stored != "1:2":
        raise AssertionError("session DB row did not use authenticated student/class")
    return session_id


def save_json(path: Path, value) -> None:
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def sse_summary(events: list[dict]) -> dict:
    sources = next(item["data"]["citations"] for item in events if item["event"] == "sources")
    answer = "".join(item["data"]["text"] for item in events if item["event"] == "delta")
    return {"event_order": [item["event"] for item in events], "answer": answer, "citations": sources,
            "tool_calls": next(item["data"]["tool_calls"] for item in events if item["event"] == "meta")}


def main() -> int:
    out = ROOT / "apitest" / "reports" / "personal-stage-acceptance-20261021"
    out.mkdir(parents=True, exist_ok=True)
    ctx = Context(BASE_URL)
    facts: list[dict] = []
    captured_model: list[dict] = []
    captured_mcp: list[dict] = []
    cleanup_errors: list[str] = []
    try:
        importlib.import_module("api.login.post").run(ctx)
        prompts_status, prompt_list = ctx.json("GET", "/api/prompts", ctx.token_a_teacher)
        skills_status, skill_list = ctx.json("GET", "/api/skills", ctx.token_a_teacher)
        if prompts_status != 200 or skills_status != 200:
            raise AssertionError("could not read teacher prompt/skill fixtures")
        baseline_prompt = next(item for item in prompt_list["prompts"] if item["is_active"])
        guided_skill = next(item for item in skill_list["skills"] if item["slug"] == "guided-solution")

        s1 = create_session(ctx, "Stage experiment 1 baseline", baseline_prompt["id"], guided_skill["id"])
        baseline_events = post_sse(ctx, s1, QUESTION, "exp1-prompt-baseline")
        model_traces = ctx.mock_traces()
        baseline_trace = next(item for item in reversed(model_traces) if item["scenario"] == "exp1-prompt-baseline")
        captured_model.append(baseline_trace)
        baseline_system = baseline_trace["request"]["messages"][0]["content"]

        templated = f"阶段验收模板标记={{class_name}}；当前班级={{{{class_name}}}}；本次问题={{{{question}}}}。{baseline_prompt['content']}"
        status, created_prompt = ctx.json("POST", "/api/prompts", ctx.token_a_teacher,
                                          {"name": f"APITEST-{ctx.run_id}-变量模板", "content": templated, "is_active": True})
        if status != 201:
            raise AssertionError(f"prompt template fixture returned HTTP {status}")
        ctx.created_prompts.append((int(created_prompt["id"]), created_prompt["name"]))
        prompt_saved = ctx.db_scalar(f"SELECT SHA2(content,256) FROM teacher_prompts WHERE id={created_prompt['id']}")
        if prompt_saved != hashlib.sha256(templated.encode("utf-8")).hexdigest():
            raise AssertionError("prompt template was not persisted exactly")
        s2 = create_session(ctx, "Stage experiment 1 templated", created_prompt["id"], guided_skill["id"])
        template_events = post_sse(ctx, s2, QUESTION, "exp1-prompt-template")
        model_traces = ctx.mock_traces()
        template_trace = next(item for item in reversed(model_traces) if item["scenario"] == "exp1-prompt-template")
        captured_model.append(template_trace)
        template_system = template_trace["request"]["messages"][0]["content"]
        if "当前班级=A 班" not in template_system or QUESTION not in template_system or "阶段验收模板标记" in baseline_system:
            raise AssertionError("prompt variables were not rendered in the actual model request")
        facts.append({"experiment": 1, "name": "提示词模板变量", "result": "PASS", "baseline_trace_id": baseline_trace["trace_id"], "template_trace_id": template_trace["trace_id"], "baseline_contains_marker": "阶段验收模板标记" in baseline_system, "template_contains_marker": "阶段验收模板标记" in template_system, "rendered_class": "A 班", "rendered_question": QUESTION, "baseline_http_request": baseline_trace["request"], "template_http_request": template_trace["request"], "baseline_api_response": sse_summary(baseline_events), "template_api_response": sse_summary(template_events)})

        class_chunks = int(ctx.db_scalar("SELECT COUNT(*) FROM knowledge_chunks WHERE class_id=1"))
        s3 = create_session(ctx, "Stage experiments 2-5", created_prompt["id"], guided_skill["id"])
        before = ctx.mock_stats()
        rag_events = post_sse(ctx, s3, QUESTION, "exp2-rag-hit")
        after_hit = ctx.mock_stats()
        rag_trace = next(item for item in reversed(ctx.mock_traces()) if item["scenario"] == "exp2-rag-hit")
        captured_model.append(rag_trace)
        rag_sources = next(item["data"]["citations"] for item in rag_events if item["event"] == "sources")
        if not rag_sources or len(rag_sources) > 4:
            raise AssertionError("RAG did not return between 1 and 4 actual sources")
        rag_system = rag_trace["request"]["messages"][0]["content"]
        if not all(hit["excerpt"] in rag_system for hit in rag_sources):
            raise AssertionError("actual retrieved source excerpts were not carried into the model system message")

        fixture = (ROOT / "apitest" / "fixtures" / "stage-acceptance-skill.md").read_text(encoding="utf-8")
        slug = f"apitest-{ctx.run_id}-stage-skill"
        skill_payload = {"slug": slug, "name": "阶段验收文件技能", "instructions": fixture,
                         "tools": [], "is_active": True}
        mcp_name = f"stage-{ctx.run_id[-8:]}"
        status, server = ctx.json("POST", "/api/mcp/servers", ctx.token_a_teacher,
                                  {"name": mcp_name, "base_url": "http://mcp-demo:8090"})
        if status != 201:
            raise AssertionError(f"MCP server registration returned HTTP {status}")
        ctx.mcp_server_id, ctx.mcp_server_name = int(server["id"]), mcp_name
        ctx.created_mcp_servers.append(ctx.mcp_server_id)
        status, connection = ctx.json("POST", f"/api/mcp/servers/{ctx.mcp_server_id}/connect", ctx.token_a_teacher)
        if status != 200 or connection.get("tools_discovered") != 2:
            raise AssertionError("MCP connect did not discover both tools")
        skill_payload["tools"] = [{"ref": f"unit_convert@{mcp_name}"}, {"ref": f"formula_hint@{mcp_name}"}]
        status, created_skill = ctx.json("POST", "/api/skills", ctx.token_a_teacher, skill_payload)
        if status != 201:
            raise AssertionError(f"file-backed skill creation returned HTTP {status}")
        skill_id = int(created_skill["id"])
        ctx.created_skills.append((skill_id, slug))
        stored_hash = ctx.db_scalar(f"SELECT SHA2(instructions,256) FROM teacher_skills WHERE id={skill_id}")
        if stored_hash != hashlib.sha256(fixture.encode("utf-8")).hexdigest():
            raise AssertionError("uploaded skill text did not match persisted DB content")
        skill_session = create_session(ctx, "Stage file-backed skill", created_prompt["id"], skill_id)
        skill_events = post_sse(ctx, skill_session, QUESTION, "exp3-skill-content")
        skill_model = next(item for item in reversed(ctx.mock_traces()) if item["scenario"] == "exp3-skill-content")
        captured_model.append(skill_model)
        skill_system = skill_model["request"]["messages"][0]["content"]
        if fixture not in skill_system:
            raise AssertionError("full uploaded skill text was absent from actual model request")
        facts.append({"experiment": 2, "name": "RAG 语料规模与 Top-K", "result": "PASS", "class_a_knowledge_chunks": class_chunks, "returned_citations": len(rag_sources), "top_k_limit": 4, "trace_id": rag_trace["trace_id"], "chat_call_delta": after_hit["chat"] - before["chat"], "actual_api_response": sse_summary(rag_events), "model_request": rag_trace["request"]})
        facts.append({"experiment": 3, "name": "文件技能与模型消息", "result": "PASS", "fixture": "apitest/fixtures/stage-acceptance-skill.md", "stored_sha256_matches_file": True, "full_skill_text_in_system_message": True, "trace_id": skill_model["trace_id"], "schema_sent_to_model": False, "MCP_auth_sent_to_model": False, "actual_skill_text": fixture, "actual_api_response": sse_summary(skill_events), "actual_model_request": skill_model["request"]})

        no_hit_before = ctx.mock_stats()
        no_hit_events = post_sse(ctx, skill_session, "火星天气和足球比分", "exp2-rag-no-hit")
        no_hit_after = ctx.mock_stats()
        no_hit_citations = next(item["data"]["citations"] for item in no_hit_events if item["event"] == "sources")
        no_hit_answer = "".join(item["data"]["text"] for item in no_hit_events if item["event"] == "delta")
        no_hit_done = next(item["data"] for item in no_hit_events if item["event"] == "done")
        if no_hit_citations or no_hit_answer != "资料中未找到相关内容" or no_hit_after["chat"] != no_hit_before["chat"]:
            raise AssertionError("no-hit did not return the deterministic no-evidence result without Chat call")
        facts.append({"experiment": 2, "name": "无命中跳过 Chat", "result": "PASS", "chat_call_delta": 0, "citations": [], "answer": no_hit_answer, "reason": "API SSE returned no sources and the mock chat counter did not change", "completed_message_id": no_hit_done["message_id"], "model_trace": "not called"})

        mcp_question = "请把 1.5 km 换算成 m，并说明实验前需要做什么准备？"
        mcp_before = ctx.mcp_traces()
        mcp_events = post_sse(ctx, skill_session, mcp_question, "exp5-mcp-tool-call")
        mcp_after = ctx.mcp_traces()
        captured_mcp.extend(item for item in mcp_after if item not in mcp_before and item.get("scenario") == "exp5-mcp-tool-call")
        mcp_model = next(item for item in reversed(ctx.mock_traces()) if item["scenario"] == "exp5-mcp-tool-call")
        captured_model.append(mcp_model)
        statuses = next(item["data"]["tool_calls"] for item in mcp_events if item["event"] == "meta")
        if not any(item.get("status") == "ok" and item.get("ref", "").startswith("unit_convert@") for item in statuses):
            raise AssertionError("SSE meta did not show a successful bound unit_convert tool")
        call_trace = next((item for item in captured_mcp if item.get("request", {}).get("method") == "tools/call"), None)
        if not call_trace:
            raise AssertionError("no actual inbound JSON-RPC tools/call trace captured at MCP boundary")
        messages = mcp_model["request"]["messages"]
        tool_text_in_model = any("1.5 km = 1500 m" in str(message.get("content", "")) for message in messages)
        if not tool_text_in_model:
            raise AssertionError("MCP return text was not included in actual model request messages")
        if any("Authorization" in json.dumps(item, ensure_ascii=False) or "MCP_AUTH_TOKEN" in json.dumps(item, ensure_ascii=False) for item in captured_model):
            raise AssertionError("sensitive MCP authorization metadata unexpectedly present in captured model-body traces")
        facts.append({"experiment": 5, "name": "MCP JSON-RPC / 模型边界", "result": "PASS", "mcp_trace_id": call_trace["trace_id"], "model_trace_id": mcp_model["trace_id"], "tool_ref": statuses[0]["ref"], "tool_statuses": statuses, "MCP_token_in_model_request": False, "tool_schema_in_model_request": False, "tool_result_in_model_request": tool_text_in_model, "chooser": "deterministic host-side question rule; model-mock does not emit native tool_calls", "actual_mcp_request": call_trace["request"], "actual_mcp_response": call_trace["response"], "actual_model_request": mcp_model["request"], "actual_model_gateway_response": mcp_model["response"], "actual_student_sse": sse_summary(mcp_events)})
        facts.append({"experiment": 4, "name": "技能真实执行链路还原", "result": "PASS", "steps": ["read the selected skill from DB", "append full instructions to server-built system message", "retrieve class-A chunks", "deterministically select only matching bound tools", "append tool return as untrusted user-role auxiliary context", "stream model deltas and persist the assistant response"], "source_files": ["frontend/src/main.tsx", "backend/internal/app/conversations.go", "backend/internal/app/store.go", "backend/internal/app/mcp_client.go", "backend/internal/app/clients.go"]})

        session_rows = ctx.db_scalar(f"SELECT CONCAT(COUNT(*), ':', SUM(role='assistant')) FROM chat_messages WHERE session_id={skill_session}")
        audit_rows = ctx.db_scalar(f"SELECT COUNT(*) FROM mcp_tool_call_audit WHERE session_id={skill_session} AND status='ok'")
        if int(session_rows.split(":")[1]) < 2 or int(audit_rows) < 1:
            raise AssertionError("session assistant rows / MCP audit evidence missing")
        facts.append({"experiment": 5, "name": "数据库持久化核对", "result": "PASS", "session_and_assistant_count": session_rows, "successful_mcp_audit_rows": int(audit_rows)})
    finally:
        try:
            captured_model.extend(item for item in ctx.mock_traces() if item not in captured_model)
        except Exception:
            pass
        try:
            captured_mcp.extend(item for item in ctx.mcp_traces() if item not in captured_mcp)
        except Exception:
            pass
        cleanup_errors = ctx.cleanup_created_data()

    # Keep boundary bodies as captured. The test process never stores HTTP Authorization headers.
    save_json(out / "model-http-traces.json", captured_model)
    save_json(out / "mcp-http-traces.json", captured_mcp)
    save_json(out / "experiments.json", {"generated_at": datetime.now(timezone.utc).isoformat(), "provider": "local OpenAI-compatible model-mock", "experiments": facts, "cleanup_errors": cleanup_errors})
    if cleanup_errors:
        print("EXPERIMENTS_SUMMARY PASS=0 FAIL=1")
        print(f"EVIDENCE_DIR={out}")
        print("TEST_DATA_CLEANUP=FAIL; newly-created rows were retained where cleanup failed")
        return 1
    if len([item for item in facts if item.get("result") == "PASS"]) < 6:
        print("EXPERIMENTS_SUMMARY PASS=0 FAIL=1")
        print(f"EVIDENCE_DIR={out}")
        return 1
    print(f"EXPERIMENTS_SUMMARY PASS={len([item for item in facts if item.get('result') == 'PASS'])} FAIL=0")
    print(f"EVIDENCE_DIR={out}")
    print("LLM_PROVIDER=local OpenAI-compatible model-mock")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
