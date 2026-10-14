from common import check_status


def run(ctx):
    assert ctx.prompt_id, "run prompt POST first"
    updated_name = ctx.values["prompt_name"] + "-updated"
    status, body = ctx.json("PUT", f"/api/prompts/{ctx.prompt_id}", ctx.token_a_teacher, {"name": updated_name, "content": "请依据可追溯资料进行引导。", "is_active": True})
    check_status(status, 200, body)
    assert body["name"] == updated_name and body["content"] == "请依据可追溯资料进行引导。"
    ctx.created_prompts = [(prompt_id, updated_name if prompt_id == ctx.prompt_id else name) for prompt_id, name in ctx.created_prompts]
    ctx.values["prompt_content"] = body["content"]
    status, denied = ctx.json("PUT", f"/api/prompts/{ctx.prompt_id}", ctx.token_a_student, {"name": "越权", "content": "越权", "is_active": True})
    check_status(status, 403, denied)
    return "教师可更新本班提示词，学生更新返回 403"
