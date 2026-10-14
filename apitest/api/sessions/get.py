from common import check_status


def run(ctx):
    assert ctx.session_id, "run session POST first"
    status, body = ctx.json("GET", "/api/sessions", ctx.token_a_student)
    check_status(status, 200, body)
    assert any(item["id"] == ctx.session_id for item in body["sessions"]), "created session is missing from list"
    status, other_user = ctx.json("GET", "/api/sessions", ctx.token_b_student)
    check_status(status, 200, other_user)
    assert all(item["id"] != ctx.session_id for item in other_user["sessions"]), "session leaked to another class/user"
    return "会话列表只包含当前用户拥有的会话"
