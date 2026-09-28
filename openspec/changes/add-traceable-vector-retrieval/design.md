## Context

第 3 课已经建立 Browser → Nginx/React → Go → MySQL 的链路，身份、角色和班级均由服务端 session 决定，教师上传的 UTF-8 原文存于 `knowledge_entries.content`。第 4 课沿用这些约束，在 Go 服务内追加切分、嵌入、检索与回答编排；浏览器仍只访问本站 `/api`。设计依据课程第 4 课“可追溯知识库检索”：正文只在 MySQL，向量只在 Qdrant，所有返回结果可回溯，无依据不得生成。

## Goals / Non-Goals

**Goals:**

- 对已入库原文生成具有稳定来源关系和位置的切片。
- 支持 keyword、vector 和默认 hybrid 三种班级隔离检索。
- 让每个命中与回答引用都可追溯到材料、知识条目、切片和位置。
- 在无可靠候选时固定拒答且不调用对话模型。
- Qdrant 故障时保留独立 keyword 能力，并对依赖向量的模式明确返回 503。

**Non-Goals:** 见 `proposal.md`；尤其不实现流式长对话、重排序框架、其他向量数据库或通用编排框架。

## Decisions

### Decision 1: MySQL 是正文与来源关系的唯一权威存储

新增表：

```text
knowledge_chunks(
  id BIGINT PRIMARY KEY,
  knowledge_entry_id BIGINT NOT NULL,
  material_id BIGINT NOT NULL,
  class_id BIGINT NOT NULL,
  chunk_index INT NOT NULL,
  chunk_text MEDIUMTEXT NOT NULL,
  start_offset INT NOT NULL,
  end_offset INT NOT NULL,
  index_status ENUM('pending','ready','failed') NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)
```

外键分别指向 `knowledge_entries`、`materials` 和 `classes`；`(knowledge_entry_id, chunk_index)` 唯一。为 `(class_id, index_status)`、`material_id` 建立普通索引，并在 `chunk_text` 上建立 `FULLTEXT ... WITH PARSER ngram`。MySQL 配置 `ngram_token_size=2` 以支持中文双字切词。

完整追溯链为 `knowledge_chunks.id → knowledge_entry_id → material_id → materials.original_filename/stored_filename`；`class_id` 冗余保存以便每一步都能强制隔离。摘录始终取自 MySQL `chunk_text`。`start_offset/end_offset` 是左闭右开的字符区间；未预处理时指向原文，启用预处理时明确指向预处理后的待切分文本，不能冒充原文件字节或字符下标。

### Decision 2: Qdrant 只保存向量与关联标识

Compose 新增内部 `qdrant` 服务、持久化 volume 和健康检查，不发布宿主机端口。collection 固定名 `campusclaw_chunks`，距离度量为 Cosine，向量维度来自服务端配置并在启动/建 collection 时严格核对。

Qdrant point id 等于 `knowledge_chunks.id`；payload 仅含 `class_id`、`material_id`、`knowledge_entry_id`、`chunk_id`、`chunk_index`。payload 禁止含 `chunk_text`、标题、文件内容或密钥。检索命中 point 后必须按 point id 和 session class_id 回 MySQL 二次核对并读取正文，Qdrant payload 不能单独作为结果来源。

### Decision 3: 三种切分策略使用同一确定性接口

- `auto`：默认策略；最大 800 字，重叠 80 字，优先在空行、换行、句号处断开。请求中额外给出的长度或预处理参数对 auto 不生效。种子材料补齐索引也使用 auto。
- `custom`：最大长度 100–2000 字，重叠比例 0%–50%，分隔符只允许换行、空行或句号；没有断点时按最大长度强制截断。可选择移除 URL/邮箱和/或将连续空白折叠为一个空格，且预处理不改写 `knowledge_entries.content`。
- `hierarchy`：按 Markdown `#`、`##`、`###` 标题分章并将标题保留在该章切片内；章节超过 800 字时再按 auto 窗口切分。

切分模块只读取正文并返回 `chunk_text/chunk_index/start_offset/end_offset`，不调用嵌入服务、不访问 Qdrant。上传与显式重建使用相同参数语义；未指定策略视为 auto。切换策略不会隐式发生，必须由本班教师显式重建。

### Decision 4: 嵌入是仅服务端的独立模块

Go 通过配置的 OpenAI-compatible `POST /embeddings` 调用嵌入模型。API base URL、model 与 API key 只存在于服务端环境；浏览器响应、日志和前端构建均不得包含密钥。每个 chunk 与检索问句分别产生一条定长浮点向量，返回维度必须等于配置值与 Qdrant collection 维度，否则本次索引或向量检索失败，不截断、不补零。

keyword 路径绝不调用嵌入接口。嵌入模块和对话生成模块是两个不同客户端/调用；生成模型不负责产生向量。

### Decision 5: 上传后的索引流程允许原文成功、索引失败

第 3 课文件、`materials`、`knowledge_entries` 原子事务先完成并保留。随后检索模块执行：

1. 读取已提交的 `knowledge_entries.content`。
2. 按选定策略切分，并以 `pending` 状态写入本批 `knowledge_chunks`。
3. 对每段调用嵌入服务。
4. 以 chunk id 为 point id 写入 Qdrant；成功后将相应 chunk 标记 `ready`。

嵌入或 Qdrant 写入失败时，材料和原文不回滚；受影响切片标记 `failed`，不留下被视为 ready 的不完整向量。只有 `ready` 切片可参与检索。

教师重建采用替换语义：先确定新策略并生成新批次，再删除该 material 的旧 Qdrant points 与旧 chunks，写入新 chunks/points；实现必须对中途失败作补偿或保留可恢复状态，保证旧正文不丢失、同一材料不会同时暴露新旧 ready 索引。重试应清理孤儿 point。

### Decision 6: keyword 检索只访问 MySQL

`keyword` 使用 `MATCH(chunk_text) AGAINST (... IN NATURAL LANGUAGE MODE)`，查询条件同时包含 `class_id = session.class_id` 与 `index_status = 'ready'`，按全文相关度降序并应用配置的绝对最低全文分数。其全过程不调用 `/embeddings`，不访问 Qdrant；因此 Qdrant 不可用时 keyword 仍正常工作。

### Decision 7: vector 检索执行向量过滤与 MySQL 二次隔离

`vector` 先嵌入 query，再请求 Qdrant，并强制 payload `class_id = session.class_id` 过滤。余弦相似度低于 `0.35` 的候选直接丢弃。候选 point id 回 MySQL 时，查询再次要求相同 session class_id 和 `index_status='ready'`；缺失、不一致或跨班记录静默丢弃。排序按余弦相似度降序，正文与出处字段从 MySQL JOIN 得到。

### Decision 8: hybrid 先绝对过滤，再以 RRF(k=60) 融合

`hybrid` 并行/独立执行 keyword 与 vector 两条完整路径；各自先应用绝对相关度阈值和班级/ready 过滤，再按各路名次计算：

```text
RRF(chunk) = Σ 1 / (60 + rank_path(chunk))
```

rank 从 1 开始，缺席路径不贡献分数。最终按 RRF 降序，平分时使用最佳单路名次、chunk id 做稳定决胜。不得直接相加、归一化后相加或把 RRF 分数当作相关性阈值。默认 mode 为 hybrid。

### Decision 9: 班级隔离在两条路径与回表阶段重复执行

搜索与问答均先经过第 3 课认证中间件。唯一有效 class_id 来自 `currentUser` session；JSON、query 或 header 中的 `class_id` 即使能被解析也必须丢弃。keyword 的 MySQL WHERE、vector 的 Qdrant filter、vector/hybrid 的 MySQL 回表都使用同一个 session class_id。

A 班查询仅在 B 班出现的内容时，对外返回 HTTP 200 和空 hits/固定无命中文案，而非 403/404，避免泄露跨班资料存在性。原材料详情/文件仍沿用第 3 课跨班与不存在同为 404 的规则。

### Decision 10: 搜索 API 返回可追溯来源

`POST /api/knowledge/search` 请求为 `{ "query": "...", "mode": "keyword|vector|hybrid" }`，mode 缺省为 hybrid。空白 query 返回 400，缺失/失效 session 返回 401，不支持的 mode 返回 400。

成功响应为 HTTP 200，包含 `mode`、`query` 和 `hits`。每个 hit 至少包含 `material_id`、`material_title`、`knowledge_entry_id`、`chunk_id`、`chunk_index`、`start_offset`、`end_offset`、取自 MySQL 的 `excerpt`、可打开的原材料 URL，以及适用于当前模式的 score/rank 元数据。无候选时 `hits: []` 并返回 `message: "资料中未找到相关内容"`。

Qdrant 不可用时 keyword 照常返回；vector 与 hybrid 返回 503，并明确表示向量检索暂不可用。不得伪造相似度、把 hybrid 偷降级为 keyword 或返回未经向量路径验证的假结果。

### Decision 11: 问答只在检索有依据后调用模型

`POST /api/ask` 接受当前用户消息及可选历史，但只用最新一条 `user` 消息作为 query。客户端提供的全部 `system` 消息丢弃；历史仅可在已有命中后作为对话上下文附加，不能参与检索或扩大班级范围。

服务端固定执行本班 hybrid 检索并取前 4 条。若无 hit，直接返回 HTTP 200：`answer: "资料中未找到相关内容"`、`citations: []`，且不得调用对话模型。有 hit 时，Go 按最终顺序给每段编号，仅向对话模型传递材料标题、切片序号、切片正文和最新问题（可附经清理的历史）；服务端 system prompt 要求只依据证据生成简短回答并用 `[1]`、`[2]` 标注。响应 citations 顺序必须与编号一致。

对话模型无数据库或 Qdrant 凭据，不得自行检索；不接收向量、原始 point、其他班级数据或客户端 system 内容。本课响应为一次性 JSON，禁止流式输出。向量依赖不可用时 `/api/ask` 返回 503，不调用对话模型。

## Failure Handling and Observability

- 索引状态变化记录 material/chunk 标识、阶段和无密钥的错误摘要；日志不得记录 API key、session cookie 或完整敏感正文。
- 远端调用使用超时、有限重试和响应大小限制；仅对安全的暂时性失败重试。
- 维度不匹配、无效向量、Qdrant 写入失败均留下 `failed` 状态并可由教师重建恢复。
- 搜索响应不得把基础设施错误伪装成空结果；依赖故障返回 503，真实低相关度/跨班过滤才返回 200 空结果。

## Deployment

Compose 保留现有 web/api/db，并新增内部 qdrant 与 `qdrant_data` named volume。API 依赖 MySQL 与 Qdrant 健康状态，但 health 设计应区分核心数据库与检索依赖，以便故障诊断。Qdrant、嵌入网关及对话网关均不作为浏览器入口；仅 web 发布宿主机端口。

## Risks / Trade-offs

- MySQL 与 Qdrant 无分布式事务：以状态机、幂等 point id、补偿删除和显式重建实现最终一致性。
- 中文 ngram token size 2 会扩大全文索引；这是课程规模下换取可验证中文 keyword 检索的选择。
- 固定 0.35 向量阈值与全文阈值可能随模型/语料变化；本课固定并可配置记录，不引入重排序调参。
- 预处理后的 offset 不能直接定位原文件；API 必须标明 offset 基准，避免伪称原始文件坐标。
- hybrid 依赖两路完整可用，Qdrant 故障返回 503；调用方可显式选择 keyword 降级，但服务端不静默改变模式。

## Migration and Verification Plan

Apply 阶段将新增幂等 SQL migration、Qdrant collection 初始化、已有/种子 `knowledge_entries` 的 auto 索引补齐及验证脚本。验证覆盖三种切分、三种检索、0.35 阈值、RRF(k=60)、MySQL 回表、双重班级隔离、伪造 class、无命中不调用模型、引用顺序、Qdrant 故障语义、失败状态和重建清理。数据库与 Qdrant volume 的持久化在不使用 `down -v` 的重启后验证。

