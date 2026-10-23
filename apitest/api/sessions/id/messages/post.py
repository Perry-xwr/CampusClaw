import json
import time
import urllib.request

from common import check_status


def _stream(ctx, session_id, content, token, disconnect_after_first_delta=False):
    request = urllib.request.Request(
        f"{ctx.base_url}/api/sessions/{session_id}/messages",
        data=json.dumps({"content": content, "class_id": 2}, ensure_ascii=False).encode("utf-8"),
        headers={
            "Authorization": f"Bearer {token}",
            "Accept": "text/event-stream",
            "Content-Type": "application/json; charset=utf-8",
        },
        method="POST",
    )
    events = {}
    event_order = []
    event_name = ""
    delta_times = []
    response = urllib.request.urlopen(request, timeout=30)
    check_status(response.status, 200)
    assert "text/event-stream" in response.headers.get("Content-Type", ""), response.headers
    try:
        for raw_line in response:
            line = raw_line.decode("utf-8").rstrip("\r\n")
            if line.startswith("event:"):
                event_name = line[6:].strip()
                if not event_order or event_order[-1] != event_name:
                    event_order.append(event_name)
            elif line.startswith("data:") and event_name:
                value = json.loads(line[5:].strip())
                events.setdefault(event_name, []).append(value)
                if event_name == "delta":
                    delta_times.append(time.perf_counter())
                    if disconnect_after_first_delta:
                        break
    finally:
        response.close()
    elapsed = (delta_times[-1] - delta_times[0]) if len(delta_times) > 1 else 0
    return events, delta_times, elapsed, event_order


def _new_session(ctx, title):
    status, body = ctx.json(
        "POST",
        "/api/sessions",
        ctx.token_a_student,
        {"title": title, "prompt_id": ctx.prompt_id, "skill_id": ctx.skill_id, "class_id": 2},
    )
    check_status(status, 201, body)
    session_id = int(body["id"])
    ctx.created_sessions.append(session_id)
    stored = ctx.db_scalar(f"SELECT CONCAT(class_id, ':', user_id) FROM chat_sessions WHERE id={session_id}")
    assert stored == "1:2", f"session persistence/class isolation mismatch: {stored!r}"
    return session_id


def run(ctx):
    assert ctx.session_id and ctx.prompt_id and ctx.skill_id, "run prompt, skill, and session tests first"

    status, invalid = ctx.json(
        "POST",
        f"/api/sessions/{ctx.session_id}/messages",
        ctx.token_a_student,
        {"content": "   ", "class_id": 2},
    )
    check_status(status, 400, invalid)
    assert invalid and invalid.get("error"), f"invalid-message response omitted error: {invalid!r}"

    events, delta_times, stream_span, event_order = _stream(
        ctx, ctx.session_id, "实验前需要做什么准备？", ctx.token_a_student
    )
    sources = events.get("sources", [{}])[0].get("citations", [])
    answer = "".join(item.get("text", "") for item in events.get("delta", []))
    assert sources and "安全培训" in sources[0]["excerpt"], f"missing retrieved citations: {sources!r}"
    assert "meta" in events and events["meta"][0].get("tool_calls") == [], f"missing empty tool-call metadata: {events.get('meta')}"
    assert event_order.index("meta") < event_order.index("sources") < event_order.index("delta") < event_order.index("done"), f"SSE event order incorrect: {event_order!r}"
    assert answer and "[1]" in answer, f"stream omitted answer/citation marker: {answer!r}"
    assert "你能指出" in answer and "安全培训。" not in answer, f"guided skill supplied an answer instead of a hint: {answer!r}"
    assert len(delta_times) > 1 and stream_span >= 0.05, (
        f"delta events were not observed incrementally: count={len(delta_times)}, span={stream_span:.3f}s"
    )
    done = events.get("done", [{}])[0]
    ctx.message_id = int(done.get("message_id", 0))
    assert ctx.message_id and done.get("answer") == answer
    ctx.values["stream_answer"] = answer
    ctx.values["stream_citations"] = sources

    tool_session = _new_session(ctx, "Lesson 6 tool-call test")
    tool_events, _, _, tool_order = _stream(ctx, tool_session, "把 1.5 km 换算成 m；实验前需要做什么准备？", ctx.token_a_student)
    assert tool_order.index("meta") < tool_order.index("sources") < tool_order.index("delta") < tool_order.index("done"), f"tool SSE event order incorrect: {tool_order!r}"
    tool_statuses = tool_events.get("meta", [{}])[0].get("tool_calls", [])
    assert tool_statuses == [{"ref": f"unit_convert@{ctx.mcp_server_name}", "status": "ok"}], f"MCP call status mismatch: {tool_statuses!r}"
    tool_done = tool_events.get("done", [{}])[0]
    audit = ctx.db_scalar(f"SELECT CONCAT(class_id, ':', user_id, ':', session_id, ':', status, ':', tool_name, ':', server_name) FROM mcp_tool_call_audit WHERE session_id={tool_session} ORDER BY id DESC LIMIT 1")
    assert audit == f"1:2:{tool_session}:ok:unit_convert:{ctx.mcp_server_name}", f"MCP audit mismatch: {audit!r}"
    persisted = ctx.db_scalar(f"SELECT JSON_UNQUOTE(JSON_EXTRACT(tool_calls, '$[0].status')) FROM chat_messages WHERE id={tool_done['message_id']}")
    assert persisted == "ok", f"tool call status was not persisted with assistant message: {persisted!r}"
    ctx.values["mcp_tool_session"] = tool_session
    stats = ctx.mock_stats()
    system_content = stats["last_chat_messages"][0]["content"]
    assert ctx.values["prompt_content"] in system_content, "teacher-managed prompt did not reach model gateway"
    assert ctx.values["guided_instructions"] in system_content, "selected guided skill did not reach model gateway"

    no_hit_session = _new_session(ctx, "Lesson 5 no-hit test")
    no_hit_before = ctx.mock_stats()["chat"]
    no_hit_events, _, _, no_hit_order = _stream(ctx, no_hit_session, "火星天气和足球比分", ctx.token_a_student)
    no_hit_citations = no_hit_events.get("sources", [{}])[0].get("citations", [])
    no_hit_answer = "".join(item.get("text", "") for item in no_hit_events.get("delta", []))
    no_hit_done = no_hit_events.get("done", [{}])[0]
    no_hit_after = ctx.mock_stats()["chat"]
    assert no_hit_citations == [] and no_hit_answer == "资料中未找到相关内容", (
        f"no-hit response mismatch: citations={no_hit_citations!r}, answer={no_hit_answer!r}"
    )
    assert no_hit_done.get("answer") == no_hit_answer and no_hit_after == no_hit_before, (
        f"no-hit incorrectly called Chat Model or was not completed: {no_hit_before}->{no_hit_after}"
    )
    assert no_hit_order.index("meta") < no_hit_order.index("sources") < no_hit_order.index("delta") < no_hit_order.index("done"), f"no-hit SSE order incorrect: {no_hit_order!r}"

    interrupted_session = _new_session(ctx, "Lesson 5 disconnect test")
    interrupted_events, interrupted_times, _, interrupted_order = _stream(
        ctx, interrupted_session, "实验前需要做什么准备？", ctx.token_a_student, disconnect_after_first_delta=True
    )
    assert interrupted_times and "sources" in interrupted_events, "disconnect test did not reach streamed evidence/delta"
    time.sleep(0.6)
    assistant_count = ctx.db_scalar(
        f"SELECT COUNT(*) FROM chat_messages WHERE session_id={interrupted_session} AND role='assistant'"
    )
    assert assistant_count == "0", f"disconnected stream persisted a completed assistant message: {assistant_count}"

    return (
        f"SSE 真正逐段到达 {len(delta_times)} 个 delta / {stream_span:.2f}s；"
        "中文、sources、done 和引用持久化通过；无命中零 Chat 调用；客户端中断未保存 completed 回答"
    )
