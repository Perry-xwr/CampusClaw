# 第六课技术方案：CampusClaw MCP 工具接入

## 运行边界

浏览器仍以现有 Bearer 会话令牌访问 Go API。Go API 是 MCP Host，使用服务端 `MCP_AUTH_TOKEN` 调用 Compose 内网中的 `mcp-demo`；浏览器、数据库记录、日志和 OpenAPI 响应都不接触该令牌。`mcp-server` 只在 Compose 网络内监听 8090，不发布宿主机端口。当前课程演示只允许精确的 `http://mcp-demo:8090` 地址，防止教师录入任意 URL 形成 SSRF。

## 注册、连接与发现

教师只能管理 Token 恢复出的当前班级服务器。连接操作先通过 Bearer MCP Token 调用 `initialize`，再调用 `tools/list`；两步均成功且工具名、描述和 inputSchema 合法后，才在同一事务中替换该服务器的工具缓存并标为 connected。失败保持 disconnected，不能沿用旧缓存。重复连接使用事务内删除后重插，避免重复记录。断开时将状态改为 disconnected 并清除工具缓存。API 重启后持久化的连接状态仍须在每次调用前通过一次 `tools/list` 重新确认工具仍存在。

## 技能绑定与鉴权

技能表增加 JSON `tools` 字段，格式为 `[{"ref":"formula_hint@mcp-demo"}]`。保存技能时逐项验证：服务器属于当前班级、处于 connected、引用语法合法且工具存在于当前发现缓存。学生创建会话时仍由服务端校验 prompt/skill 的班级归属和 active 状态；发送消息时再次校验 session 所属 user/class、技能和每个工具引用。客户端提交的 class_id 不参与授权。

现有 ModelClient / model-mock 不支持原生 tool_calls。为避免把确定性演示逻辑冒充模型自主决策，本版本由 Go Host 的小型规则选择器决定是否调用已绑定工具：只有消息显式包含单位换算表达式，才选 `unit_convert`；只有包含“公式提示”或“formula hint”，才选 `formula_hint`。模型只负责后续依据材料组织回答。每条学生消息最多执行 3 个 MCP 调用，最多 2 轮；本规则选择器当前仅产生一轮，保留总轮数硬上限。达到上限不再发起工具调用。未选择工具时照常走知识库问答。

## 调用、结果信任与失败处理

Host 重新发现并核验工具后才发起 `tools/call`，发送 JSON-RPC id/method/params，并在 Authorization 中附加仅服务端读取的令牌。工具结果作为不可信数据，不拼入 system 指令；提示模型只能把它当作辅助数据并继续以本班材料作为知识依据。工具超时、断连、拒绝或坏响应不会把知识检索/SSE 主路径改成成功的工具调用；Host 返回 failed 状态并留下审计，继续依据有效材料回答。工具引用无权、未发现、未连接时记录 denied，不静默跳过。

## 次数限制和审计

限制按每条学生消息计算：最多 3 次 tools/call、最多 2 轮，单次 MCP HTTP 超时 5 秒，参数/响应各限制 64 KiB，单条文本结果限制 4 KiB。计数在服务端调用循环中递增，客户端不能指定或重置。审计表保留 class_id、server_id/name、tool_name、user_id、session_id、message_id、status、error_code 和 created_at；不保存令牌、请求参数或完整工具输出。每次允许、拒绝、连接失败均记录。审计写入失败则不执行工具调用，避免出现无法追踪的外部动作。

## SSE 事件

有证据时事件顺序为 `meta`、`sources`、零到多个 `delta`、`done`；无证据时不调用 Chat Model，发送 `meta`（空 tool_calls）、空 `sources`、提示文本 `delta`、`done`。`meta` 数据格式：

```json
{"tool_calls":[{"ref":"unit_convert@mcp-demo","status":"ok"}]}
```

状态取 `ok`、`denied`、`error`；不得包含令牌、工具参数或秘密。断连/模型错误使用现有 `error` 事件，不写入 completed 的 assistant 消息。客户端断开时请求 context 取消，后续工具调用和模型流也随之停止。

## 四项交叉验证证据

在课程验证记录中为每项保存截图或脱敏日志片段（时间、用例、请求路径、状态码/事件和结论；不得包含令牌）：

1. 班级和工具隔离：A 班教师连接/发现/绑定成功；B 班学生无法读取 A 班服务器；伪造 class_id 不改变服务端范围。
2. Token 不回传：注册和列表响应只提供 `has_token` 布尔值；API、前端、OpenAPI 和日志中无 MCP Token 明文。
3. 断开即拒绝：断开后同一技能工具调用产生 denied 状态与审计，且不访问 MCP 服务；重新连接后重新发现方可调用。
4. 审计范围：调用日志记录身份、班级、工具和结果状态；教师只能读本班审计，学生和跨班用户不能访问。

## 配置与兼容

Compose 将同一个宿主机 `MCP_AUTH_TOKEN` 环境值分别注入 API 与 MCP 服务。`.env.example` 只说明变量用途，不提供示例令牌；缺少变量时 MCP `/mcp` 拒绝请求，连接/调用安全失败，既有 Lesson 3–5 非 MCP 路径保持可用。数据库用新迁移新增服务器、发现工具及调用审计表，并在技能表添加 JSON 绑定字段；不清理任何旧数据。
