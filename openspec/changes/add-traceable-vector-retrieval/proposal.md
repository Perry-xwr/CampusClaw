## Why

第 3 课 change `add-auth-rbac-class-knowledge` 已完成登录、服务端会话、角色校验、班级隔离、材料上传及 `knowledge_entries` 原文入库。本课在这套安全边界和原文数据之上增加可追溯检索，使学生和教师能够用自然语言从本班材料中找到有依据的片段，并让每条命中和简短回答都能回到材料、知识条目、切片及原始位置。

## What Changes

- 将 `knowledge_entries` 正文按 auto、custom 或 hierarchy 策略切成 `knowledge_chunks`；默认 auto 为最大 800 字、重叠 80 字。
- 在 MySQL 保存切片正文、班级、来源关系、序号、字符区间和索引状态；在 Qdrant `campusclaw_chunks` collection 只保存向量和标识 payload，point id 与 `knowledge_chunks.id` 相同。
- 由 Go 服务通过兼容 OpenAI 的 `/embeddings` 接口生成向量；密钥只保留在服务端，并校验向量维度与 collection 一致。
- 提供 keyword、vector、hybrid 三种检索模式，默认 hybrid；混合检索在各路径完成绝对阈值过滤后使用 RRF（k=60）按名次融合，不相加原始分数。
- 每条检索结果返回材料标题、material/entry/chunk 标识、切片序号、字符区间、MySQL 正文摘录及可打开的原材料来源。
- 提供非流式简短问答：只用最新一条用户消息执行本班 hybrid 检索并取前 4 条；有依据才调用对话模型并以 `[1]`、`[2]` 标注引用，无依据时直接返回固定文案 `资料中未找到相关内容`，且不调用模型。
- 扩展 Compose 加入仅供服务端访问且不发布宿主机端口的 Qdrant，并增加索引、检索、故障及班级隔离的自动验证。

## Non-goals（非目标）

- 不重做或替换第 3 课的认证、RBAC、班级隔离、材料上传与原文入库。
- 不实现流式输出、长对话记忆、通用 AI Chat、Agent、工具调用或网页搜索。
- 不实现 MCP、智能作业或其他第 5 课之后的能力。
- 不实现交叉编码器重排序、语义切分、父子块检索、增量段落更新或模型微调。
- 不引入 Milvus、pgvector、Chroma、Weaviate、Elasticsearch、LangChain、LlamaIndex 或 Dify；本课只使用 MySQL、Qdrant 与 Go 自行编排。
- 不实现跨班知识共享或公共知识库，也不允许前端传入 `class_id` 控制检索范围。
- 不把正文、摘录、密钥或向量分量暴露给 Qdrant payload 或浏览器。
- 不改变上传权限：上传和重建索引仍为教师操作，学生仅能检索和提问本班资料。
- 不实现生产级高可用、多副本、分布式事务、在线模型供应商切换或运维控制台。

## Capabilities

### New Capabilities

- `knowledge-retrieval`: 可配置切分与重建、服务端嵌入、Qdrant 向量索引、keyword/vector/hybrid 检索、班级双重隔离、可追溯来源及有依据才生成的简短问答。

### Modified Capabilities

- 无。第 3 课已有能力保持不变；本 change 只在已入库的知识正文之后追加检索能力。

## Impact

- 新增 MySQL `knowledge_chunks` 表、中文 ngram FULLTEXT 索引及必要的来源/状态索引。
- Compose 新增内部 Qdrant 服务和持久化 volume；web 仍是唯一对宿主机发布端口的服务。
- Go API 新增切分、嵌入、向量索引、检索融合、问答编排与索引重建模块，以及 `POST /api/knowledge/search`、`POST /api/ask` 和教师重建入口。
- 材料上传事务成功后触发切分与索引；索引失败不删除已提交的材料或原文，而以 `failed` 状态保留诊断和重试依据。
- 增加嵌入/对话网关、模型、维度、Qdrant 地址和阈值等服务端环境变量，但任何密钥均不得进入前端构建产物或响应。
