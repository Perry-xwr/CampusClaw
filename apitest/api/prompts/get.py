from common import check_status


def run(ctx):
    status, body = ctx.json("GET", "/api/prompts", ctx.token_a_teacher)
    check_status(status, 200, body)
    assert body.get("prompts"), "teacher prompt list is empty"
    status, student = ctx.json("GET", "/api/prompts", ctx.token_a_student)
    check_status(status, 200, student)
    assert all(item["is_active"] for item in student["prompts"]), "student received inactive prompt"
    status, class_b = ctx.json("GET", "/api/prompts", ctx.token_b_student)
    check_status(status, 200, class_b)
    foreign_id = class_b["prompts"][0]["id"]
    status, denied = ctx.json("PUT", f"/api/prompts/{foreign_id}", ctx.token_a_teacher, {"name": "跨班修改", "content": "禁止", "is_active": True})
    check_status(status, 404, denied)
    return "教师和学生均可读取本班提示词，学生仅看到启用项"
