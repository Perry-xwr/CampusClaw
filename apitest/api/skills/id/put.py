from common import check_status


def run(ctx):
    skill_id = ctx.values.get("test_skill_id")
    assert skill_id, "run skill POST first"
    status, protected = ctx.json("PUT", f"/api/skills/{ctx.skill_id}", ctx.token_a_teacher, {"slug": "disabled-guidance", "name": "解题引导", "instructions": "直接回答。", "is_active": True})
    check_status(status, 400, protected)
    updated_slug = ctx.values["test_skill_slug"] + "-updated"
    status, body = ctx.json("PUT", f"/api/skills/{skill_id}", ctx.token_a_teacher, {"slug": updated_slug, "name": "测试技能更新", "instructions": "只提问引导，不给答案。", "is_active": True})
    check_status(status, 200, body)
    ctx.created_skills = [(created_id, updated_slug if created_id == skill_id else slug) for created_id, slug in ctx.created_skills]
    status, denied = ctx.json("PUT", f"/api/skills/{skill_id}", ctx.token_a_student, {"slug": "forged", "name": "越权", "instructions": "越权"})
    check_status(status, 403, denied)
    return "教师可更新班级技能，学生更新被拒绝"
