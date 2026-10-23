from common import check_status


def run(ctx):
    # The server cannot be deleted while this run's test skill still references it.
    status, skills = ctx.json("GET", "/api/skills", ctx.token_a_teacher)
    check_status(status, 200, skills)
    skill = next(item for item in skills["skills"] if item["id"] == ctx.values["test_skill_id"])
    status, updated = ctx.json("PUT", f"/api/skills/{skill['id']}", ctx.token_a_teacher, {"slug": skill["slug"], "name": skill["name"], "instructions": skill["instructions"], "tools": [], "is_active": skill["is_active"]})
    check_status(status, 200, updated)
    status, _, _ = ctx.request("DELETE", f"/api/mcp/servers/{ctx.mcp_server_id}", token=ctx.token_a_teacher)
    check_status(status, 204)
    remaining = ctx.db_scalar(f"SELECT COUNT(*) FROM mcp_servers WHERE id={ctx.mcp_server_id}")
    assert remaining == "0", f"MCP server was not deleted: {remaining}"
    ctx.created_mcp_servers.remove(ctx.mcp_server_id)
    return "已断开且无技能绑定的本轮 MCP 服务可安全删除"
