from common import check_status
import json


def run(ctx):
    status, body = ctx.json("POST", f"/api/mcp/servers/{ctx.mcp_server_id}/disconnect", ctx.token_a_teacher)
    check_status(status, 200, body)
    assert body["connected"] is False and body["tools_cleared"] is True
    count = ctx.db_scalar(f"SELECT COUNT(*) FROM mcp_server_tools WHERE server_id={ctx.mcp_server_id}")
    connected = ctx.db_scalar(f"SELECT is_connected FROM mcp_servers WHERE id={ctx.mcp_server_id}")
    assert count == "0" and connected == "0", f"disconnect did not clear cached tools: {count}/{connected}"
    status, missing = ctx.json("GET", f"/api/mcp/servers/{ctx.mcp_server_id}/tools", ctx.token_a_teacher)
    check_status(status, 404, missing)
    session_id = ctx.values.get("mcp_tool_session")
    assert session_id, "MCP tool session missing"
    status, headers, stream = ctx.request("POST", f"/api/sessions/{session_id}/messages", ctx.token_a_student, {"content": "2 km 转 m；实验前需要什么准备？"})
    check_status(status, 200)
    assert "text/event-stream" in headers.get("Content-Type", "")
    events = {}
    event_name = ""
    for line in stream.decode("utf-8").splitlines():
        if line.startswith("event:"):
            event_name = line[6:].strip()
        elif line.startswith("data:") and event_name:
            events.setdefault(event_name, []).append(json.loads(line[5:].strip()))
    statuses = events.get("meta", [{}])[0].get("tool_calls", [])
    assert statuses == [{"ref": f"unit_convert@{ctx.mcp_server_name}", "status": "denied"}], f"disconnected tool was not denied: {statuses!r}"
    denied = ctx.db_scalar(f"SELECT CONCAT(status, ':', error_code) FROM mcp_tool_call_audit WHERE session_id={session_id} ORDER BY id DESC LIMIT 1")
    assert denied == "denied:tool_not_connected_or_discovered", f"disconnect denial not audited: {denied!r}"
    assert events.get("done"), "tool denial unexpectedly broke the normal answer path"
    return "断开后缓存清空；绑定工具调用被拒绝并审计，知识问答仍正常完成"
