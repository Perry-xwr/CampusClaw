import json

from common import check_status


def run(ctx):
    assert ctx.session_id and ctx.message_id, "run message POST first"
    status, body = ctx.json("GET", f"/api/sessions/{ctx.session_id}/messages", ctx.token_a_student)
    check_status(status, 200, body)
    messages = body.get("messages", [])
    assert len(messages) >= 2 and messages[-2]["role"] == "user" and messages[-1]["role"] == "assistant", messages
    assert messages[-1]["content"] == ctx.values["stream_answer"]
    assert messages[-1]["citations"] == ctx.values["stream_citations"]
    count = ctx.db_scalar(f"SELECT COUNT(*) FROM chat_messages WHERE session_id={ctx.session_id} AND class_id=1 AND role IN ('user','assistant')")
    assert count == "2", f"expected two messages persisted, got {count}"
    citation_count = ctx.db_scalar(f"SELECT JSON_LENGTH(citations) FROM chat_messages WHERE id={ctx.message_id}")
    assert int(citation_count) == len(ctx.values["stream_citations"]), f"stored citations mismatch: {citation_count}"
    status, foreign = ctx.json("GET", f"/api/sessions/{ctx.session_id}/messages", ctx.token_b_student)
    check_status(status, 404, foreign)
    return "流式消息、引用和数据库记录一致；其他班级用户访问返回 404"
