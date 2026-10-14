from common import check_status


def run(ctx):
    # Keep the DB round-trip assertion ASCII-only because mysql's output encoding
    # inside docker compose exec is not reliably UTF-8 on Windows hosts.
    name = "APITEST-" + ctx.run_id + "-prompt"
    content = "基于本班证据回答，逐步引导思考。"
    status, body = ctx.json("POST", "/api/prompts", ctx.token_a_teacher, {"name": name, "content": content})
    check_status(status, 201, body)
    ctx.prompt_id = int(body["id"])
    ctx.created_prompts.append((ctx.prompt_id, name))
    ctx.values["prompt_name"] = name
    stored = ctx.db_scalar(f"SELECT CONCAT(class_id, ':', created_by, ':', name) FROM teacher_prompts WHERE id={ctx.prompt_id}")
    assert stored == f"1:1:{name}", f"prompt DB mismatch: {stored!r}"
    status, forbidden = ctx.json("POST", "/api/prompts", ctx.token_a_student, {"name": name + "-student", "content": content})
    check_status(status, 403, forbidden)
    status, foreign = ctx.json("GET", "/api/prompts", ctx.token_b_student)
    check_status(status, 200, foreign)
    assert all(item["name"] != name for item in foreign["prompts"]), "A-class prompt leaked to B-class"
    return "提示词写入 MySQL，class_id/created_by 正确；学生写入被拒绝且 B 班不可见"
