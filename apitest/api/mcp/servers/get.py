from common import check_status


def run(ctx):
    status, body = ctx.json("GET", "/api/mcp/servers", ctx.token_a_teacher)
    check_status(status, 200, body)
    assert isinstance(body.get("servers"), list)
    assert all("token" not in server and "secret" not in server for server in body["servers"])
    status, denied = ctx.json("GET", "/api/mcp/servers", ctx.token_a_student)
    check_status(status, 403, denied)
    status, class_b = ctx.json("GET", "/api/mcp/servers", ctx.token_b_student)
    check_status(status, 403, class_b)
    return "教师可列出本班 MCP 服务；学生无权读取管理接口，响应不含令牌字段"
