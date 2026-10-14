from common import check_status


def run(ctx):
    status, body = ctx.json("POST", "/api/knowledge/search", ctx.token_a_student, {"query": "实验前安全培训", "mode": "hybrid", "class_id": 2})
    check_status(status, 200, body)
    hits = body.get("hits", [])
    assert hits, "expected seeded A-class material to be retrievable"
    assert all("量子通信" not in hit.get("material_title", "") for hit in hits), "cross-class source leaked"
    assert any("安全培训" in hit.get("excerpt", "") for hit in hits), "response did not contain expected relevant excerpt"
    return f"响应包含 {len(hits)} 条本班证据；伪造 class_id 未改变检索范围"
