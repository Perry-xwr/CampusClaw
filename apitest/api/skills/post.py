from common import check_status


def run(ctx):
    slug = "apitest-" + ctx.run_id + "-skill"
    status, body = ctx.json("POST", "/api/skills", ctx.token_a_teacher, {"slug": slug, "name": "测试技能", "instructions": "只提供一个思考提示，不输出最终答案。"})
    check_status(status, 201, body)
    ctx.values["test_skill_id"] = int(body["id"])
    ctx.created_skills.append((int(body["id"]), slug))
    ctx.values["test_skill_slug"] = slug
    stored = ctx.db_scalar(f"SELECT CONCAT(class_id, ':', created_by, ':', slug) FROM teacher_skills WHERE id={body['id']}")
    assert stored == f"1:1:{slug}", f"skill DB mismatch: {stored!r}"
    status, forbidden = ctx.json("POST", "/api/skills", ctx.token_a_student, {"slug": slug + "-student", "name": "越权", "instructions": "越权"})
    check_status(status, 403, forbidden)
    return "教师创建技能持久化成功，学生创建返回 403"
