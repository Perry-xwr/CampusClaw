from common import check_status


def run(ctx):
    for username, password, role, class_id, attr in [
        ("teacher_a", "TeacherA123!", "teacher", 1, "token_a_teacher"),
        ("student_a1", "StudentA123!", "student", 1, "token_a_student"),
        ("student_b1", "StudentB123!", "student", 2, "token_b_student"),
    ]:
        status, body = ctx.json("POST", "/api/login", payload={"username": username, "password": password})
        check_status(status, 200, body)
        assert body["user"]["role"] == role and body["user"]["class_id"] == class_id, body
        assert body.get("token"), "login response omitted token"
        setattr(ctx, attr, body["token"])
        ctx.issued_tokens.append(body["token"])
    status, body = ctx.json("POST", "/api/login", payload={"username": "student_a1", "password": "invalid"})
    check_status(status, 401, body)
    return "教师/学生登录身份正确，错误密码返回 401"
