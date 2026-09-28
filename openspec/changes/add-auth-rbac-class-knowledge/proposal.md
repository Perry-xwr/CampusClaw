## Why

CampusClaw needs a small but trustworthy foundation before adding AI features. Iteration 1 establishes identity, teacher/student authorization, class-scoped data access, and ingestion of teacher-provided materials so later retrieval features have isolated, attributable source data.

## What Changes

- Add username/password login and server-side sessions.
- Add teacher and student roles enforced by the API.
- Scope every material and knowledge query to the authenticated user's class.
- Allow teachers to upload non-empty UTF-8 `.txt` and `.md` files; persist the file, a `materials` row, and a linked `knowledge_entries` row atomically.
- Add a minimal React UI for login, material browsing, teacher upload, current-user display, and logout.
- Seed two classes and distinguishable users/materials for isolation verification.
- Deliver Nginx, Go API, and MySQL 8 as `web`, `api`, and `db` Docker Compose services, with persistent database and upload volumes.
- Add an unauthenticated `GET /health` endpoint and reproducible positive and failure-path verification.

## Non-goals（非目标）

- AI Chat or any general-purpose conversation feature.
- RAG retrieval, embeddings, ranking, or vector databases.
- Intelligent lesson planning or question generation.
- Assignment submission or grading.
- Registration, password recovery, OAuth, or SSO.
- Kubernetes, high availability, multiple application replicas, or production operations.
- Material editing/deletion, binary document parsing, or virus scanning.

## Capabilities

### New Capabilities

- `auth`: password login, logout, current session, and unauthenticated rejection.
- `authorization`: server-side teacher/student permissions.
- `class-isolation`: server-derived class scoping for list, detail, and file access.
- `materials`: validated teacher upload and read-only material access.
- `knowledge-ingestion`: linked material and knowledge records with failure atomicity.

### Modified Capabilities

（无；本项目为全新仓库。）

## Impact

- Introduces `frontend/`, `backend/`, `docker/`, and Compose configuration.
- Introduces MySQL tables `classes`, `users`, `sessions`, `materials`, and `knowledge_entries` plus repeatable seed data.
- Introduces persistent named volumes for MySQL data and uploaded source files.
- The host only needs Git, Docker Desktop, and Docker Compose; Go and Node builds occur inside containers.

