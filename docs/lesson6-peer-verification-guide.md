# CampusClaw 第六课同伴验证操作指南

本指南供真实小组同学在浏览器中完成四项交叉验证。以下四项是 CampusClaw 当前采用的验证方案，**尚未证实与老师课件中的四项验证原文完全一致**。不要借用他人身份、伪造截图或把自动化报告当作同伴证据。

## 准备

- 浏览器打开 [CampusClaw](http://localhost:8080/)；账号密码使用课程环境提供的凭据，不在截图或记录中写密码、Bearer Token 或 MCP Token。
- 教师账号：`teacher_a`（A 班）。学生账号：`student_a1`（A 班）、`student_b1`（B 班）。仅记录用户名、角色和班级，不记录密码。
- 需要查看原始 HTTP/SSE 时，可使用浏览器开发者工具的 Network 面板。不要复制或截图 Authorization 请求头、Storage 中的登录 Token。
- 建议每项由不同同学操作；记录操作者昵称/组别和实际时间。操作前不要删除已有服务、技能、材料或审计记录。

## A. 教师登记、连接并发现工具

**操作角色：** A 班教师 `teacher_a`。

**入口：** `http://localhost:8080/` 登录后，在“AI 学习会话”中展开“教师管理提示词与技能”。

**步骤：**

1. 在“MCP 工具服务”中登记服务名 `campus-tools`，地址填写 `http://mcp-demo:8090`，提交。
2. 确认服务出现在列表，点击“连接并发现工具”。
3. 展开“已发现工具”区域（或教师技能工具选项），确认发现了 `formula_hint` 和 `unit_convert`，且各自有说明/schema。

**预期：** 服务显示已连接；发现两个工具。响应仅显示是否配置了服务端 Token，不展示 Token 明文。

**证据：** 服务名称、连接状态和两个工具清单同屏截图；另保存一段脱敏 Network 记录（请求路径、HTTP 状态、响应中的工具名/schema）。不得截取 Authorization 头。

**记录：** `apitest/reports/lesson6-local/peer-evidence/A-record.md`；截图建议命名 `A-YYYYMMDD-HHMM-操作者.png`。

## B. 绑定技能并由学生会话触发工具

**操作角色：** 教师 `teacher_a` 完成绑定；学生 `student_a1` 完成提问。

**入口：** 教师页面的“教师管理提示词与技能”→“学习技能”；学生页面的“AI 学习会话”。

**步骤：**

1. 教师新建或编辑启用技能，勾选已经发现的 `unit_convert@campus-tools`，保存。绑定由服务器校验。
2. 学生登录后，在“技能”中选中该技能，点击“新建会话”。
3. 提问：`把 1.5 km 转换成 m；实验前需要做什么准备？`
4. 检查回答旁的工具状态，以及材料引用卡片。需要原始事件时，在 Network 中查看消息请求的 EventStream，确认 `meta` 的 `tool_calls` 状态为 `ok`，其后有 `sources`、多个 `delta` 和 `done`。

**预期：** UI 显示 `unit_convert@campus-tools` 成功；回答有本班知识来源。SSE 里出现真实工具状态而非仅有打字机动画。

**证据：** 技能绑定界面、学生回答和引用、SSE 事件顺序/工具状态的截图或脱敏日志；仅保留事件名、工具引用、状态和时间，不保留凭据或完整敏感请求头。

**记录：** `apitest/reports/lesson6-local/peer-evidence/B-record.md`；截图建议命名 `B-YYYYMMDD-HHMM-操作者.png`。

## C. 验证教师权限与班级隔离

**操作角色：** 教师 `teacher_a`、学生 `student_a1`、不同班级学生 `student_b1`。

**入口：** `http://localhost:8080/`，分别登录各账号；开发者工具 Console 可用于核对 API 状态码。

**步骤：**

1. `teacher_a` 登录，确认教师管理提示词/技能和 MCP 管理入口可见。
2. `student_a1` 登录，确认教师管理/MCP 管理入口不可见。可在 Console 发起 `fetch('/api/mcp/servers', {headers:{Authorization:'Bearer '+localStorage.getItem('campusclaw_token')}})`，只记录 HTTP 状态和响应，不打印 Token；预期 `403`。
3. `student_b1` 登录，确认班级显示 B 班、材料列表为 B 班内容。使用该会话检查 A 班材料文件 ID `1`：`fetch('/api/materials/1/file', {headers:{Authorization:'Bearer '+localStorage.getItem('campusclaw_token')}}).then(r=>({status:r.status}))`，预期 `404`。同时确认 A 班独有材料不出现在其检索结果中。
4. 如学生对 MCP 管理路由的请求返回 `403`，将其记录为角色权限证据；跨班材料 ID 的 `404` 和按班检索结果用于记录数据隔离证据。不要把单独一个 `403` 解释成已独立证明所有跨班行为。

**预期：** 教师功能只对教师显示并由后端保护；学生不能读取教师 MCP 管理 API；B 班学生看不到/不能读取 A 班材料，客户端伪造班级字段不能改变服务端范围。

**证据：** 各账号的角色/班级界面、管理功能可见性、脱敏的状态码和检索结果摘要。截屏前确认没有密码、Authorization 头或 Token。

**记录：** `apitest/reports/lesson6-local/peer-evidence/C-record.md`；截图建议命名 `C-YYYYMMDD-HHMM-操作者.png`。

## D. 查看审计并验证断开后拒绝调用

**操作角色：** 教师 `teacher_a` 查看审计；学生 `student_a1` 触发断开后的调用。

**入口：** 教师管理区域的“MCP 工具服务”与“最近工具调用审计”；学生的“AI 学习会话”。

**步骤：**

1. 确认 B 项已产生一次 `unit_convert` 成功调用，并在教师页面审计区域看到学生、工具和 `ok` 状态。
2. 教师点击 `campus-tools` 的“断开并清缓存”。
3. 学生再次使用仍绑定该工具的技能提问同类换算问题。
4. 检查学生侧 `meta.tool_calls` 为 `denied`（不能是 `ok`），问答主路径可按知识资料继续回答；返回教师页刷新审计，确认出现拒绝记录。

**预期：** 断开后发现缓存清空，工具调用被拒绝并写入审计；审计结果不含 Token、参数或完整输出。断开前成功记录仍可见。

**证据：** 断开前成功审计、断开后服务状态、学生侧 denied 状态、教师侧 denied 审计记录的截图或脱敏事件日志。

**记录：** `apitest/reports/lesson6-local/peer-evidence/D-record.md`；截图建议命名 `D-YYYYMMDD-HHMM-操作者.png`。

## PASS / FAIL 记录约定

每位操作者应在对应 `*-record.md` 中填写实际日期时间、操作者、角色/班级、结果、证据文件名与结论。只有步骤实际完成且证据可核对时才能写 `PASS`；失败写 `FAIL` 并保留错误状态/原因；未执行保持 `NOT TESTED`。不要将自动化 apitest 的 PASS 复制成同伴 PASS。
