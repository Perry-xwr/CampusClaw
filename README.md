# CampusClaw · 班级知识库与 AI 学习助手

CampusClaw 是课程用班级知识管理与 AI 学习平台，采用 React + TypeScript、Go、MySQL、Qdrant 和 Docker Compose。系统包含 Bearer Token 认证、教师/学生 RBAC、班级隔离、可追溯 RAG 问答、提示词与 Skill 管理、MCP 工具接入和 SSE 会话。项目用于课程演示与学习，不应直接视为生产部署模板。

## 功能

- bcrypt 密码登录、Bearer 随机 token、数据库服务端 session、登出和当前用户信息
- teacher 可上传非空 UTF-8 `.txt` / `.md`；student 只读且上传 API 返回 403
- 列表、详情和文件下载全部按 session 中的班级过滤
- 文件、`materials` 和 `knowledge_entries` 同步入库，失败时回滚并清理文件
- 双班种子数据，可验证查询参数伪造与跨班 ID 访问
- React + TypeScript 演示界面，Go API，MySQL 8，Nginx
- `GET /health` 数据库健康检查
- auto/custom/hierarchy 三种正文切分、Qdrant cosine 向量索引
- keyword、vector、hybrid 检索；hybrid 使用 RRF（k=60）
- 每条命中返回材料、知识条目、切片、字符区间和 MySQL 正文摘录
- `/api/ask` 仅在本班检索有依据后生成非流式简短回答

### Lesson 5–6：对话、Skill 与 MCP

- 教师管理提示词模板和学习 Skill；学生创建会话并通过 SSE 接收回答及引用。
- 浏览器导入 `.md` / `.txt` Skill 时读取文件文本，再把指令文本保存到 CampusClaw，不上传本地文件路径。
- 同仓 `mcp-server/` 提供 `formula_hint`、`unit_convert`。Host 按技能绑定与确定性规则选择工具，不使用模型原生 function calling。
- 服务端检查身份、班级、绑定及调用限制，并记录工具审计。
- Swagger UI：<http://localhost:8080/docs/>；OpenAPI：`frontend/public/docs/openapi.yaml`。

### Chat 模型配置

- 默认 Chat 使用本地 `model-mock`，适用于离线开发和自动化验收。
- 可通过 `CHAT_BASE_URL`、`CHAT_MODEL`、`CHAT_API_KEY` 配置 OpenAI-compatible Chat API；Compose 也兼容映射 `GLM_BASE_URL`、`GLM_MODEL`、`GLM_API_KEY`。智谱示例见 `.env.example`。
- 用户必须自行获取并在本机 `.env` 配置 API Key。仓库不提供真实密钥；不要提交 `.env`、Key 或带认证头的 Trace。
- Chat 与 Embedding 分别配置。默认 Embedding 使用 `model-mock`、维度 16；更换 Embedding 前需匹配维度并评估索引，不能只修改 Chat 配置。
- 真实模型请求可能产生费用。自动化测试默认使用 mock，不代表真实模型或所有 MCP 场景均已完成最终验收。

本项目仍不做通用 AI Chat、流式多轮对话、智能备课/命题、作业提交/批改、注册/找回密码、OAuth/SSO、Kubernetes 或多副本部署。

## 架构

```text
Browser → web (Nginx + React) → api (Go) → db (MySQL 8：原文/切片)
                                  ├→ qdrant（向量/标识）
                                  ├→ embedding provider
                                  ├→ chat provider
                                  └→ uploads_data volume
```

只有 `web` 向宿主机发布端口。API、MySQL、Qdrant 和本地 model-mock 均不映射宿主机端口。`db_data`、`uploads_data` 与 `qdrant_data` 是命名 volume。

## 目录

```text
backend/       Go API、SQL migration、测试、Dockerfile
frontend/      React/Vite UI、Nginx 配置、Dockerfile
openspec/      Iteration 1 proposal/design/specs/tasks
model-mock/    仅用于开发、自动验收和课堂演示的确定性模型接口
scripts/       失败路径自动验收脚本
docs/          现场验收记录
compose.yaml   web/api/db 三服务定义
```

## 环境要求

宿主机只强制要求：

- Git
- Docker Desktop（Windows 上须启动 Docker Desktop）
- Docker Compose v2（`docker compose`）

Go 与 Node.js 在 Docker 构建阶段提供，宿主机不必安装。Docker Desktop 官方下载：<https://www.docker.com/products/docker-desktop/>。

## 启动

PowerShell：

```powershell
Copy-Item .env.example .env
docker compose up --build -d
docker compose ps
Invoke-RestMethod http://localhost:8080/health
```

浏览器访问 <http://localhost:8080>。若修改 `.env` 的 `WEB_PORT`，请使用相应端口。

首次使用前应修改 `.env` 中两个数据库密码。`.env` 已被 Git 忽略。

## API 鉴权

`POST /api/login` 成功后返回随机 session token 和用户信息：

```json
{
  "token": "<raw-session-token>",
  "user": {
    "id": 1,
    "username": "teacher_a",
    "role": "teacher",
    "class_id": 1,
    "class_name": "A 班"
  }
}
```

除登录和健康检查外，API 请求必须携带 `Authorization: Bearer <token>`。数据库只保存 token 的 SHA-256 哈希；后端使用 token 查询服务端 session，并以 session 关联的用户、角色和班级作为权限依据，客户端提交的 `role` 或 `class_id` 不会改变身份。前端为课程演示将 token 保存在 `localStorage`；这不是对生产环境最佳安全方案的声明。

## 本地演示账号

仅限本地开发数据库中的种子账号使用。首次启动后，请查看本地开发配置或重置为自有的开发凭据；不要在公网部署中启用或复用课程种子账号。数据库保存密码哈希。仓库不发布可用于公网的账号密码。

## 演示与验收

1. 用 `teacher_a` 登录，上传一个小于等于 1 MiB 的 UTF-8 `.md` 或 `.txt`，刷新后应看到并可下载。
2. 登出后用 `student_a1` 登录：能看到 A 班材料，但没有上传表单；直接调用上传 API 返回 403。
3. 用 `student_b1` 登录，只能看到 B 班种子材料；A 班材料不会出现在列表中。
4. A 班 session 请求 `/api/materials?class_id=2` 仍只返回 A 班；直接访问 B 班种子材料 ID `2` 返回 404。
5. 运行完整失败路径脚本：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify.ps1
```

脚本覆盖未登录 401、错误密码、student 403 与无副作用、伪造 role、伪造 class、跨班 404、`.exe`、空文件、非 UTF-8、超限文件和教师成功入库，并对所有拒绝路径核对数据库及文件计数。

容器内运行后端测试：

```powershell
docker build --target build -t campusclaw-api-test ./backend
docker run --rm campusclaw-api-test go test ./...
```

说明：最终 API 镜像不包含 Go 工具链；日常验证可直接执行 `docker build --target build`。若需要交互测试，亦可使用 `golang:1.24-alpine` 挂载 `backend/`。

## 持久化验证

教师上传后记录材料 ID，再执行：

```powershell
docker compose down
docker compose up -d
```

重新登录，确认材料仍在列表且文件仍可下载。`docker compose down` 不删除 volume。

停止服务：

```powershell
docker compose down
```

不要用 `docker compose down -v` 作为普通停止命令；`-v` 会删除数据库、上传文件和向量数据卷，造成数据丢失。

## 健康检查

`GET /health` 不需要登录。数据库可用时返回 HTTP 200：

```json
{"status":"ok"}
```

数据库不可用时返回 HTTP 503。

## Lesson 4：可追溯知识库检索

正文与向量分开保存：`knowledge_entries.content` 和 `knowledge_chunks.chunk_text` 位于 MySQL；Qdrant collection `campusclaw_chunks` 只保存定长向量及 `class_id/material_id/knowledge_entry_id/chunk_id/chunk_index`。Qdrant point id 与 `knowledge_chunks.id` 相同，返回摘录必须回 MySQL 获取。

三种检索模式：

- `keyword`：只访问 MySQL FULLTEXT ngram，不调用 embedding 或 Qdrant。
- `vector`：query embedding 后按 session 班级过滤 Qdrant，余弦相似度低于 0.35 丢弃，再按 chunk id 和相同班级回 MySQL。
- `hybrid`：默认模式；两路各自过滤后用 RRF（k=60）融合名次，不相加原始分数。

班级范围始终来自 `Bearer token → server-side session → current user → class_id`。客户端提交的任何 `class_id` 都不会改变 MySQL、Qdrant 或回表范围。无候选时固定返回 `资料中未找到相关内容`。

### Search API

```http
POST /api/knowledge/search
Authorization: Bearer <token>
Content-Type: application/json

{"query":"做实验之前需要先进行什么准备？","mode":"hybrid"}
```

响应的每个 hit 包含材料标题、material/entry/chunk ID、chunk 序号、字符区间、MySQL 摘录、rank/score 和受 Bearer 保护的材料 URL。

### Ask API

```http
POST /api/ask
Authorization: Bearer <token>
Content-Type: application/json

{"messages":[{"role":"user","content":"做实验之前需要先进行什么准备？"}]}
```

API 只用最新一条 user 消息执行本班 hybrid 检索并取前 4 条。有命中才调用 chat provider，并让 `[1]`、`[2]` 与 citations 顺序一致；无命中时不调用 chat，直接返回固定文案和空 citations。客户端 system 消息会被丢弃。

### 模型 Provider

默认 Compose 使用内部 `model-mock`。它是固定维度、确定性、OpenAI-compatible 的课程测试服务，不是真实大模型，不应被描述为真实 AI 能力。它不发布宿主机端口，业务代码只通过统一 HTTP client 调用。

真实 OpenAI-compatible Chat 与 Embedding 是独立服务配置。Chat 可配置 `CHAT_BASE_URL`、`CHAT_API_KEY`、`CHAT_MODEL`；Compose 也支持由 `GLM_BASE_URL`、`GLM_API_KEY`、`GLM_MODEL` 映射智谱 Chat。Embedding 默认继续使用 `model-mock`，不要为切换 Chat 而更改向量维度。复制 `.env.example` 为 `.env` 后，由使用者自行填写密钥；不要提交 `.env` 或真实 Key。

如需切换 Embedding provider，才单独配置：

```dotenv
EMBEDDING_BASE_URL=https://provider.example/v1
EMBEDDING_API_KEY=replace-me
EMBEDDING_MODEL=embedding-model
EMBEDDING_DIMENSION=1536
CHAT_BASE_URL=https://provider.example/v1
CHAT_API_KEY=replace-me
CHAT_MODEL=chat-model
```

Base URL 不含 `/embeddings` 或 `/chat/completions` 后缀。修改 embedding 维度前，必须使用匹配维度的新 Qdrant collection；已有 collection 维度不一致时 API 会明确拒绝启动。密钥只进入 API 容器环境，不得写入仓库、前端或响应。

### 故障语义与演示

Qdrant 不可用时 keyword 仍工作，vector、hybrid 和 `/api/ask` 返回 503，不静默降级。课堂演示可依次：以 A 班登录；keyword 搜索“太阳能”；vector 搜索“做实验之前需要先进行什么准备？”；观察 hybrid 的 RRF 排名与来源；查询 B 班独有“量子通信”得到固定无命中；最后调用 `/api/ask` 查看 `[1]` citation。

完整回归与 Lesson 4 验收：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify.ps1
```

脚本会分别报告 Lesson 3 和 Lesson 4 结果，并执行 Qdrant 故障恢复及不带 `-v` 的持久化重启。
