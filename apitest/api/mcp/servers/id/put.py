from common import check_status


def run(ctx):
    assert ctx.mcp_server_id
    ctx.mcp_server_name += "-v2"
    status, body = ctx.json("PUT", f"/api/mcp/servers/{ctx.mcp_server_id}", ctx.token_a_teacher, {"name": ctx.mcp_server_name, "base_url": "http://mcp-demo:8090"})
    check_status(status, 200, body)
    assert body["name"] == ctx.mcp_server_name and not body["connected"]
    stored = ctx.db_scalar(f"SELECT name FROM mcp_servers WHERE id={ctx.mcp_server_id} AND class_id=1")
    assert stored == ctx.mcp_server_name
    return "断开状态下可修改服务名称，持久化值一致"
