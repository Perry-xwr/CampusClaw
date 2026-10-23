from common import check_status


def run(ctx):
    status, body = ctx.json("GET", "/api/mcp/tool-calls", ctx.token_a_teacher)
    check_status(status, 200, body)
    assert isinstance(body.get("tool_calls"), list)
    assert all("arguments" not in row and "output" not in row and "token" not in row for row in body["tool_calls"])
    matching = [row for row in body["tool_calls"] if row["server_name"] == ctx.mcp_server_name and row["username"] == "student_a1"]
    assert any(row["tool_name"] == "unit_convert" and row["status"] == "ok" for row in matching), "successful MCP call audit was not listed"
    assert any(row["tool_name"] == "unit_convert" and row["status"] == "denied" for row in matching), "disconnected denial audit was not listed"
    status, denied = ctx.json("GET", "/api/mcp/tool-calls", ctx.token_a_student)
    check_status(status, 403, denied)
    return "审计接口教师可读且不泄漏参数/输出/令牌，学生访问为 403"
