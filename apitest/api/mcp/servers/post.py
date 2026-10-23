from common import check_status


def run(ctx):
    ctx.mcp_server_name = "apitest-" + ctx.run_id.replace("-", "")
    base_url = "http://mcp-demo:8090"
    status, body = ctx.json("POST", "/api/mcp/servers", ctx.token_a_teacher, {"name": ctx.mcp_server_name, "base_url": base_url})
    check_status(status, 201, body)
    ctx.mcp_server_id = int(body["id"])
    ctx.created_mcp_servers.append(ctx.mcp_server_id)
    assert body["name"] == ctx.mcp_server_name and body["base_url"] == base_url
    assert body["connected"] is False and "token" not in body
    stored = ctx.db_scalar(f"SELECT CONCAT(class_id, ':', created_by, ':', name, ':', base_url) FROM mcp_servers WHERE id={ctx.mcp_server_id}")
    assert stored == f"1:1:{ctx.mcp_server_name}:{base_url}", f"MCP server persistence mismatch: {stored!r}"
    status, denied = ctx.json("POST", "/api/mcp/servers", ctx.token_a_student, {"name": ctx.mcp_server_name + "-student", "base_url": base_url})
    check_status(status, 403, denied)
    status, blocked = ctx.json("POST", "/api/mcp/servers", ctx.token_a_teacher, {"name": "bad-target", "base_url": "http://127.0.0.1:3306"})
    check_status(status, 400, blocked)
    return "教师注册仅接受 Compose 内网 MCP URL，数据库按当前班级和创建者保存"
