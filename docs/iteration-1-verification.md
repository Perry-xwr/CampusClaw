# CampusClaw Iteration 1 验收记录

记录日期：2026-09-23（Asia/Shanghai）

## 环境事实

| 项目 | 当前结果 |
|---|---|
| 项目目录 | `E:\CampusClaw` |
| Git | 已安装：`git version 2.52.0.windows.1` |
| Docker Desktop / Docker CLI | 已安装并运行：Docker 29.8.0；CLI `C:\Users\29238\AppData\Local\Programs\DockerDesktop\resources\bin\docker.exe` |
| Docker Compose | 已安装：v5.5.1 |
| Go | 宿主机未安装；设计为容器内构建 |
| Node.js | Codex 内置 Node `v24.19.0` 可见，但 npm 不可用；设计为容器内构建 |
| OpenSpec CLI | 宿主未安装；已用临时官方 CLI 完成 strict validate |

结论：Docker daemon（`desktop-linux`）可访问，三服务已经实际构建和运行。Go/Node 不要求在宿主机安装。

## 设计审核

- 规约四件套及五个 capability delta 已创建。
- 认证、授权、班级隔离分别落在 session 查询、teacher middleware、带 `class_id` 的 SQL 条件中。
- 任何客户端 `role` / `class_id` 都不会成为授权或隔离依据。
- 跨班与不存在材料统一返回 `404 {"error":"material not found"}`。
- 上传只允许 `.txt` / `.md`、非空 UTF-8、最大 1 MiB，并使用临时文件 + DB transaction + 失败清理。
- 设计判断：满足 Apply 条件；Compose 已实测。
- 临时官方 OpenSpec CLI 严格校验通过：`Change 'add-auth-rbac-class-knowledge' is valid`。
- 临时官方 Go `go1.27.1` 已运行 `gofmt` 与 `go test ./...`，全部通过。
- 临时官方 Node/npm 已运行 TypeScript 检查与 Vite 生产构建，全部通过。

## 当前实现清单

- Go API 路由：login、logout、me、materials list/detail/file/upload、health
- MySQL schema、bcrypt 双班种子账号与双班材料/knowledge 数据
- React 登录、材料列表、下载、教师上传、登出及当前用户显示
- Nginx 同源代理和 SPA fallback
- `web` / `api` / `db` Compose、healthcheck、依赖条件、DB/upload volumes
- Go 上传验证单元测试与 PowerShell 失败路径验收脚本
- README 与 OpenSpec change

## 运行证据

- `docker compose config --quiet`：exit 0。
- `docker compose up --build -d`：api/web 镜像构建成功；db/api healthy；web 对外发布 8080。
- `GET /health`：数据库正常时 200；停止 db 时 503；恢复后再次 200。
- `scripts/verify.ps1`：13 PASS，0 FAIL。
- 持久化：material ID 4、class 1、knowledge 1 条、stored filename 和 SHA-256 在 `down`/`up -d` 前后完全一致；下载返回 200。
- Docker Hub 鉴权端点在本机持续走不可达 IPv6，基础镜像改用 `docker.m.daocloud.io/library/*` 代理；镜像仍为官方 library 镜像及固定 digest。

安装并启动 Docker Desktop 后，执行：

```powershell
Copy-Item .env.example .env
docker compose up --build -d
docker compose ps
Invoke-RestMethod http://localhost:8080/health
powershell -ExecutionPolicy Bypass -File scripts/verify.ps1
```

## 失败路径结果

| 场景 | 预期 | 当前证据 |
|---|---:|---|
| 未登录 API | 401 | PASS |
| 错误密码 | 401、session 不增加 | PASS |
| student 上传 / 伪造 teacher | 403 且三项 count 不变 | PASS |
| A 查询 `class_id=B` | 仍只有 A | PASS |
| A 访问 B material ID/file | 与不存在资源同形 404 | PASS |
| A teacher 提交 `class_id=B` | 数据库仍为 A | PASS |
| `.exe` | 400，无残留 | PASS |
| 空文件 | 400，无残留 | PASS |
| 非 UTF-8 | 400，无残留 | PASS |
| 超限 | 413，无残留 | PASS |
| 合法 teacher 上传 | material/knowledge/file 各 +1 | PASS |

## 尚未完成

- `4.5`：现有 Go 单测覆盖上传验证，但 auth/RBAC/isolation 的完整覆盖当前由 `verify.ps1` 提供，不是 Go 测试。
- `5.2`、`5.3`：浏览器自动化 helper 连续重置失败，未取得 UI 登录/登出与教师/学生控件的真实交互证据；API 和生产构建均已通过。

## 两分钟老师现场演示

1. **0:00–0:20**：展示 `openspec/changes/add-auth-rbac-class-knowledge/` 的 proposal、design、specs、tasks，强调 Non-goals 与服务端信任边界。
2. **0:20–0:35**：运行 `docker compose ps` 和 `Invoke-RestMethod http://localhost:8080/health`，展示三服务与 200 health。
3. **0:35–1:00**：用 `teacher_a` 登录，展示 A 班身份，上传 `demo.md`，刷新列表并下载。
4. **1:00–1:20**：用 `student_a1` 登录，展示材料可读但无上传入口；运行验收脚本展示直接 POST 仍为 403 且三项计数不变。
5. **1:20–1:40**：展示 `?class_id=2` 无法越权、B 班 ID 返回 404；切换 `student_b1` 展示仅 B 班数据。
6. **1:40–2:00**：展示 `.exe` / 空文件 / 超限失败结果，再说明 `docker compose down` → `up -d` 后上传仍在，证明两个 volume 持久化。
