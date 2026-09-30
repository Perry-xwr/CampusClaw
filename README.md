# CampusClaw · Iteration 1

CampusClaw 是课程用教研智能体平台。本迭代只交付后续智能能力的安全底座：登录、教师/学生权限、班级隔离、文本材料上传，以及写入 `knowledge_entries`。规约位于 `openspec/changes/add-auth-rbac-class-knowledge/`。

## 功能

- bcrypt 密码登录、Bearer 随机 token、数据库服务端 session、登出和当前用户信息
- teacher 可上传非空 UTF-8 `.txt` / `.md`；student 只读且上传 API 返回 403
- 列表、详情和文件下载全部按 session 中的班级过滤
- 文件、`materials` 和 `knowledge_entries` 同步入库，失败时回滚并清理文件
- 双班种子数据，可验证查询参数伪造与跨班 ID 访问
- React + TypeScript 演示界面，Go API，MySQL 8，Nginx
- `GET /health` 数据库健康检查

本次明确不做 AI Chat、RAG、智能备课/命题、作业提交/批改、注册/找回密码、OAuth/SSO、Kubernetes 或多副本部署。

## 架构

```text
Browser → web (Nginx + React) → api (Go) → db (MySQL 8)
                                  │
                                  └→ uploads_data volume
```

只有 `web` 向宿主机发布端口。MySQL 的 3306 不映射到宿主机。`db_data` 与 `uploads_data` 是命名 volume。

## 目录

```text
backend/       Go API、SQL migration、测试、Dockerfile
frontend/      React/Vite UI、Nginx 配置、Dockerfile
openspec/      Iteration 1 proposal/design/specs/tasks
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

## 测试账号

| 用户名 | 密码 | 角色 | 班级 |
|---|---|---|---|
| `teacher_a` | `TeacherA123!` | teacher | A 班 |
| `student_a1` | `StudentA123!` | student | A 班 |
| `student_b1` | `StudentB123!` | student | B 班 |

数据库只保存运行时生成的 bcrypt 哈希，不保存表中的明文密码。以上密码仅是课程演示凭据。

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

彻底清理数据库与上传文件（不可恢复，仅在明确需要重置演示数据时执行）：

```powershell
docker compose down -v
```

## 健康检查

`GET /health` 不需要登录。数据库可用时返回 HTTP 200：

```json
{"status":"ok"}
```

数据库不可用时返回 HTTP 503。
