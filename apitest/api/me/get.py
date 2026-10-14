from common import check_status


def run(ctx):
    status, body = ctx.json("GET", "/api/me")
    check_status(status, 401, body)
    assert body and body.get("error"), f"missing unauthenticated error body: {body!r}"

    status, body = ctx.json("GET", "/api/me", "not-a-valid-token")
    check_status(status, 401, body)
    assert body and body.get("error"), f"missing invalid-token error body: {body!r}"

    status, body = ctx.json("GET", "/api/me", ctx.token_a_student)
    check_status(status, 200, body)
    assert body.get("username") == "student_a1" and body.get("role") == "student" and body.get("class_id") == 1, body
    return "缺少/无效 Token 均为 401；有效 Bearer Token 恢复服务端用户身份"
