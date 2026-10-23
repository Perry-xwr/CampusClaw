from common import check_status


def run(ctx):
    assert ctx.mcp_server_id
    status, body = ctx.json("POST", f"/api/mcp/servers/{ctx.mcp_server_id}/connect", ctx.token_a_teacher)
    check_status(status, 200, body)
    assert body["connected"] is True and body["tools_discovered"] == 2 and body["has_token"] is True
    stored = ctx.db_scalar(f"SELECT CONCAT(is_connected, ':', (SELECT COUNT(*) FROM mcp_server_tools WHERE server_id={ctx.mcp_server_id})) FROM mcp_servers WHERE id={ctx.mcp_server_id}")
    assert stored == "1:2", f"discovery cache mismatch: {stored!r}"
    status, repeat = ctx.json("POST", f"/api/mcp/servers/{ctx.mcp_server_id}/connect", ctx.token_a_teacher)
    check_status(status, 200, repeat)
    stored_count = ctx.db_scalar(f"SELECT COUNT(*) FROM mcp_server_tools WHERE server_id={ctx.mcp_server_id}")
    assert stored_count == "2", f"repeated connect duplicated cache rows: {stored_count}"
    return "initialize/tools/list 成功；重复连接替换缓存而未重复插入"
