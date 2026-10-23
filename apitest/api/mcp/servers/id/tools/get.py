from common import check_status


def run(ctx):
    status, body = ctx.json("GET", f"/api/mcp/servers/{ctx.mcp_server_id}/tools", ctx.token_a_teacher)
    check_status(status, 200, body)
    tools = body.get("tools", [])
    assert {tool["name"] for tool in tools} == {"formula_hint", "unit_convert"}
    assert all(tool["input_schema"].get("type") == "object" for tool in tools)
    return "教师发现工具清单包含名称、说明及对象 inputSchema"
