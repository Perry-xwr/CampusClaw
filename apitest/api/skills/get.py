from common import check_status


def run(ctx):
    status, body = ctx.json("GET", "/api/skills", ctx.token_a_teacher)
    check_status(status, 200, body)
    skill = next((item for item in body.get("skills", []) if item["slug"] == "guided-solution"), None)
    assert skill, "guided-solution skill is missing"
    assert "不要直接给出最终答案" in skill["instructions"], "guided-solution does not prohibit final answers"
    ctx.skill_id = int(skill["id"])
    ctx.default_skill_id = ctx.skill_id
    ctx.values["guided_instructions"] = skill["instructions"]
    status, student = ctx.json("GET", "/api/skills", ctx.token_a_student)
    check_status(status, 200, student)
    assert any(item["id"] == ctx.skill_id for item in student["skills"])
    status, class_b = ctx.json("GET", "/api/skills", ctx.token_b_student)
    check_status(status, 200, class_b)
    status, denied = ctx.json("PUT", f"/api/skills/{class_b['skills'][0]['id']}", ctx.token_a_teacher, {"slug": "cross-class", "name": "跨班技能", "instructions": "禁止", "is_active": True})
    check_status(status, 404, denied)
    return "本班包含学生可用的解题引导技能，提示只引导不直接给最终答案"
