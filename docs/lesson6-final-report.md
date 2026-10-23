# CampusClaw 第六课最终验收报告

## 1. 结论与范围

本报告整理 CampusClaw 第六课代码和 2026-10-21 的本地一键验收结果。自动化验收已通过；四项真实同伴交叉验证仍未执行，因此不能据此认定课程所需的同伴证据已经完成。下面区分了实际运行结果、构建缓存和未做的检查。

## 2. Host、Client、Server 架构

- 浏览器是既有 CampusClaw 前端，通过用户登录后得到的 Bearer Token 调用 Go API。
- Go API 在 MCP 流程中充当 MCP Host，并使用内嵌的 Go MCP Client（`backend/internal/app/mcp_client.go`）访问 MCP Server。
- 同仓 `mcp-server/` 通过 Docker Compose 运行在 `mcp-demo` 服务，Compose 内部地址为 `http://mcp-demo:8090`；没有发布 MCP 容器宿主机端口。
- API 和 MCP Server 运行时使用同一 `MCP_AUTH_TOKEN`。这是服务间凭据，与浏览器用户 Token 分开；不会由前端提交、在 API 响应中返回或写入报告。Server 缺少配置时 `/mcp` 安全失败。

## 3. MCP JSON-RPC 与工具

MCP Server 提供 `GET /health` 和受 Bearer 保护的 `POST /mcp`，实现 JSON-RPC 2.0 的 `initialize`、`tools/list`、`tools/call`。本地 MCP Server 单元测试 5/5 通过；apitest 的教师连接流程也真实执行 initialize 和 tools/list，并核对缓存的两个工具及其 schema。

工具如下：

| 工具 | 功能 | 限制 |
|---|---|---|
| `formula_hint` | 按支持的概念给出公式线索和思考方向，不输出最终答案 | 仅支持代码列出的主题 |
| `unit_convert` | 在支持的公制长度、质量单位之间做确定性换算 | 不同量纲、未知单位或非法数值会返回 JSON-RPC 参数错误 |

教师把工具绑定到技能后，学生发消息时 Go Host 按明确的规则匹配问题并通过 MCP Client 发起远端 `tools/call`。当前 model-mock 不支持原生 tool call；这里是服务端规则选择，不是 LLM 自主选择工具。工具结果以低信任辅助数据加入问答上下文，不拼入 system 指令。

## 4. 教师管理、发现、技能绑定和审计

教师管理界面可登记服务、连接、断开、查看已发现工具和近期调用审计。服务端只允许配置的 Compose 内部 MCP 地址，避免任意 URL 带来的 SSRF 风险。连接时先 initialize，再 tools/list；通过校验后将工具清单持久化缓存。重复连接会替换缓存；断开会清缓存。

技能以 `工具名@服务器名` 形式绑定，例如 `unit_convert@campus-tools`。服务端保存时校验当前用户教师角色、当前班级、服务器连接状态和发现缓存；调用时还会重新校验。审计表记录班级、服务器、工具、用户、会话、消息、状态和错误分类，不记录凭据、工具参数或完整输出。教师只读取当前班级审计，学生无权访问管理接口。

数据库迁移 [004_mcp_tools.sql](/E:/CampusClaw/backend/migrations/004_mcp_tools.sql) 增加 `teacher_skills.tools`、`chat_messages.tool_calls`、`mcp_servers`、`mcp_server_tools` 和 `mcp_tool_call_audit`。apitest 实际核对了发现缓存、技能绑定、调用审计及断开后的拒绝记录。

## 5. 鉴权、班级隔离与调用限制

浏览器仍以当前登录用户的 Bearer Token 访问 API。角色和 `class_id` 来自服务端恢复出的用户身份；客户端伪造 `class_id` 不作为授权依据。MCP 服务间另用运行时 `MCP_AUTH_TOKEN`，不复用学生 Token。

每条消息最多发起 3 次工具调用，代码保留最多 2 轮的上限；当前选择器和执行循环实际只进入首轮。MCP Client 的 HTTP 超时为 5 秒，工具返回文本最多 4 KiB，请求和 JSON-RPC 响应读取限制为 64 KiB。连接或工具调用失败会形成 `error`/`denied` 状态和审计，不会伪报成功；知识问答主路径仍可继续处理有效资料。

## 6. SSE、工具状态与引用

成功问答的 SSE 顺序为 `meta`、`sources`、多个 `delta`、`done`。`meta` 中的 `tool_calls` 展示工具引用及 `ok`、`denied` 或 `error` 状态；assistant 消息也持久化工具状态。知识引用按检索结果顺序带回材料、chunk、字符偏移和 excerpt。无命中时返回固定提示、空 citations，不调用 Chat Model。

本轮 apitest 实测 SSE 在约 0.17 秒内逐段收到 17 个 delta，并核对中文、sources、done、引用/消息数据库记录、工具状态和审计；也验证了无命中零 Chat 调用及客户端中断不保存 completed 回答。

## 7. Swagger 与 apitest

Swagger UI 地址：[http://localhost:8080/docs/](http://localhost:8080/docs/)。OpenAPI 文件地址：[http://localhost:8080/docs/openapi.yaml](http://localhost:8080/docs/openapi.yaml)。验收后以只读 HTTP 请求确认 UI 返回 200（HTML）且 OpenAPI YAML 返回 200。此为 HTTP 可访问性检查，未记录真实浏览器截图或第三方 CDN 的可视渲染检查。

本次 apitest：**21 PASS / 0 FAIL**；其中 Lesson 6 MCP 测试 **13 PASS / 0 FAIL**。完整 HTML：[api-test-20261021101035-86771010.html](/E:/CampusClaw/apitest/reports/api-test-20261021101035-86771010.html)。测试用例覆盖登录、检索、提示词、技能、MCP 服务、会话、SSE、工具审计、权限和跨班访问，并检查关键 MySQL 状态。runner 报告本轮测试数据清理无失败；没有清理历史用户、材料或 Docker 数据卷。

## 8. Docker、构建与持久化

2026-10-21 的验收构建了 `mcp-demo`、`api` 和 `web` 镜像，并确认 `db`、`qdrant`、`model-mock`、`mcp-demo`、`api`、`web` 六个服务最终均为 healthy。`/health` 返回 200。

API Dockerfile 中的 `go test ./...` 和 `go build` 在本轮 BuildKit 输出为 **CACHED**；因此本轮确认 API 镜像构建步骤成功，但不能说这两个 Go 命令在本轮被重新执行。独立 Go 测试是否另行运行不由该缓存命中证明。

`scripts/verify.ps1` 实际完成 Lesson 3/4 回归及 down/up 持久化检查，输出 Lesson 3 **22 PASS / 0 FAIL**、Lesson 4 **22 PASS / 0 FAIL**。持久化检查核对 MySQL chunks、Qdrant points、检索结果和引用在无 `-v` 的 down/up 后仍可用。未执行 volume 删除。

## 9. 本次实际验收汇总

| 验收项 | 实际结果 | 证据 |
|---|---:|---|
| 一键验收 | 46 PASS / 0 FAIL / 0 NOT RUN | `apitest/reports/lesson6-local/summary.md` |
| apitest | 21 PASS / 0 FAIL | 上述 summary 与 HTML 报告 |
| Lesson 6 MCP apitest | 13 PASS / 0 FAIL | summary 的逐项测试与 MCP 汇总 |
| Lesson 3 回归 | 22 PASS / 0 FAIL | `apitest/reports/lesson6-local/acceptance.log` |
| Lesson 4 回归 | 22 PASS / 0 FAIL | 同上，包含持久化检查 |
| 六个服务 | 全部 healthy | summary 的 Compose 服务状态表 |
| MCP Server 独立单测 | 5/5 PASS | 本地 `python -m unittest discover -s mcp-server -p test_server.py` 结果 |
| Swagger/OpenAPI HTTP | 两个 URL 均 200 | 本次只读 HTTP 检查 |

详细日志为 `apitest/reports/lesson6-local/acceptance.log`。该目录和 HTML 报告均由 `.gitignore` 忽略。报告显示本次 MCP Token 仅在验收进程内生成并传给 Compose，没有写入 `.env`、源码、镜像构建参数或报告。

## 10. 尚未完成

四项同伴验证仍为 **NOT TESTED**。请依照 [同伴验证指南](lesson6-peer-verification-guide.md) 由真实同学操作并保存脱敏证据。当前本地方案是项目的操作清单；尚未确认与老师课件的四项原文完全一致。自动化 PASS 不替代同伴操作，也不代表这四项课程证据已经提交。
