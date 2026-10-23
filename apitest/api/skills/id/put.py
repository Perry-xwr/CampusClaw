from common import check_status


def run(ctx):
    skill_id = ctx.values.get("test_skill_id")
    assert skill_id, "run skill POST first"
    status, protected = ctx.json("PUT", f"/api/skills/{ctx.default_skill_id}", ctx.token_a_teacher, {"slug": "disabled-guidance", "name": "解题引导", "instructions": "直接回答。", "is_active": True})
    check_status(status, 400, protected)
    updated_slug = ctx.values["test_skill_slug"] + "-updated"
    updated_instructions = "只提问引导，不给答案。禁止直接给出最终答案。"
    status, body = ctx.json("PUT", f"/api/skills/{skill_id}", ctx.token_a_teacher, {"slug": updated_slug, "name": "测试技能更新", "instructions": updated_instructions, "tools": [{"ref": f"unit_convert@{ctx.mcp_server_name}"}], "is_active": True})
    check_status(status, 200, body)
    ctx.created_skills = [(created_id, updated_slug if created_id == skill_id else slug) for created_id, slug in ctx.created_skills]
    ctx.values["guided_instructions"] = updated_instructions
    stored = ctx.db_scalar(f"SELECT JSON_UNQUOTE(JSON_EXTRACT(tools, '$[0].ref')) FROM teacher_skills WHERE id={skill_id}")
    assert stored == f"unit_convert@{ctx.mcp_server_name}", f"skill binding update mismatch: {stored!r}"
    status, denied = ctx.json("PUT", f"/api/skills/{skill_id}", ctx.token_a_student, {"slug": "forged", "name": "越权", "instructions": "越权"})
    check_status(status, 403, denied)
    return "教师可更新班级技能，学生更新被拒绝"
