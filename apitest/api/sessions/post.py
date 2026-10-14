from common import check_status


def run(ctx):
    assert ctx.prompt_id and ctx.skill_id, "run prompt and skill tests first"
    title = "APITEST-" + ctx.run_id + "-session"
    status, body = ctx.json("POST", "/api/sessions", ctx.token_a_student, {"title": title, "prompt_id": ctx.prompt_id, "skill_id": ctx.skill_id, "class_id": 2})
    check_status(status, 201, body)
    ctx.session_id = int(body["id"])
    ctx.created_sessions.append(ctx.session_id)
    stored = ctx.db_scalar(f"SELECT CONCAT(class_id, ':', user_id, ':', prompt_id, ':', skill_id) FROM chat_sessions WHERE id={ctx.session_id}")
    assert stored == f"1:2:{ctx.prompt_id}:{ctx.skill_id}", f"session ownership/class mismatch: {stored!r}"
    status, foreign_skill = ctx.json("POST", "/api/sessions", ctx.token_a_student, {"prompt_id": ctx.prompt_id, "skill_id": 1_000_000, "class_id": 2})
    check_status(status, 404, foreign_skill)
    return "会话属于当前学生及 A 班；伪造 class_id 被忽略，越班技能不可选"
