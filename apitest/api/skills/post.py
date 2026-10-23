from common import check_status


def run(ctx):
    slug = "apitest-" + ctx.run_id + "-skill"
    instructions = "只提供一个思考提示，不输出最终答案。禁止直接给出最终答案。"
    binding = {"ref": f"unit_convert@{ctx.mcp_server_name}"}
    status, body = ctx.json("POST", "/api/skills", ctx.token_a_teacher, {"slug": slug, "name": "测试技能", "instructions": instructions, "tools": [binding]})
    check_status(status, 201, body)
    ctx.values["test_skill_id"] = int(body["id"])
    ctx.created_skills.append((int(body["id"]), slug))
    ctx.values["test_skill_slug"] = slug
    ctx.values["guided_instructions"] = instructions
    ctx.skill_id = int(body["id"])
    assert body["tools"] == [binding]
    stored_tools = ctx.db_scalar(f"SELECT JSON_UNQUOTE(JSON_EXTRACT(tools, '$[0].ref')) FROM teacher_skills WHERE id={body['id']}")
    assert stored_tools == binding["ref"], f"skill tool binding not persisted: {stored_tools!r}"
    status, invalid = ctx.json("POST", "/api/skills", ctx.token_a_teacher, {"slug": slug + "-invalid", "name": "无效绑定", "instructions": "验证", "tools": [{"ref": "unit_convert@other-class"}]})
    check_status(status, 400, invalid)
    stored = ctx.db_scalar(f"SELECT CONCAT(class_id, ':', created_by, ':', slug) FROM teacher_skills WHERE id={body['id']}")
    assert stored == f"1:1:{slug}", f"skill DB mismatch: {stored!r}"
    status, forbidden = ctx.json("POST", "/api/skills", ctx.token_a_student, {"slug": slug + "-student", "name": "越权", "instructions": "越权"})
    check_status(status, 403, forbidden)
    return "教师创建技能持久化成功，学生创建返回 403"
