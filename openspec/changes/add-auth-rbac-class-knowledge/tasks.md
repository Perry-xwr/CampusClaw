## 1. Repository foundation

- [x] 1.1 Create repository configuration, environment example, directories, and Git ignore rules. Verify: inspected the project tree and ran `git -c safe.directory=E:/CampusClaw status --short --branch`.
- [x] 1.2 Write and review the OpenSpec proposal, design, delta specs, and this task plan. Verify: temporary official CLI ran `openspec validate add-auth-rbac-class-knowledge --strict` successfully.

## 2. Database and backend foundation

- [x] 2.1 Add versioned MySQL schema for classes, users, sessions, materials, and knowledge entries. Verify: fresh named DB volume initialized all tables and constraints during Compose startup.
- [x] 2.2 Add idempotent two-class seed data with bcrypt password hashes. Verify: queried 2 classes, three class-scoped users, `$2a$` 60-character hashes, no plaintext passwords, and seeded materials/knowledge.
- [x] 2.3 Create the Go API configuration, DB connection/retry, migration runner, and JSON error helpers. Verify: `go test ./...` passed and the built API started successfully against MySQL.
- [x] 2.4 Implement public `GET /health` with database readiness. Verify: live DB returned 200; after `docker compose stop db`, health returned 503; recovery returned 200.

## 3. Authentication and authorization

- [x] 3.1 Implement login with bcrypt and opaque DB sessions. Verify: correct login set an HttpOnly cookie; wrong password returned 401 and session count stayed unchanged.
- [x] 3.2 Implement authentication middleware and `GET /api/me`. Verify: no cookie returned 401; teacher cookie returned `teacher_a`, teacher role, class 1.
- [x] 3.3 Implement logout and session revocation. Verify: after logout the former session received 401 from `/api/me`.
- [x] 3.4 Implement teacher-only authorization middleware. Verify: student upload with forged `role=teacher` returned 403 and all three counts stayed unchanged.

## 4. Class-scoped materials and ingestion

- [x] 4.1 Implement class-scoped material list. Verify: A list excluded B material even with `?class_id=2`.
- [x] 4.2 Implement class-scoped detail and file endpoints. Verify: cross-class and nonexistent detail/file requests all returned identical 404 JSON.
- [x] 4.3 Implement upload validation for extension, size, non-empty content, and UTF-8. Verify: `.exe`, empty, invalid UTF-8, and oversized requests returned 400/413 with unchanged DB/file counts.
- [x] 4.4 Implement transactional material and knowledge insertion plus staged file cleanup. Verify: valid teacher upload increased material, knowledge, and file counts by exactly one and forced class 1.
- [ ] 4.5 Add backend tests for authentication, RBAC, isolation, upload success, and all required failure paths. Verify: `go test ./...` exits 0.

## 5. Frontend

- [x] 5.1 Create the React + TypeScript + Vite application and API client. Verify: temporary official Node/npm ran `npm run build` successfully; container build remains covered by 6.1.
- [ ] 5.2 Implement login, current-user display, logout, and protected navigation. Verify: successful login reaches the material page and logout returns to login.
- [ ] 5.3 Implement material list/download and teacher upload UI; hide upload for students. Verify: teacher can upload and refresh; student sees list without upload control.

## 6. Reproducible deployment

- [x] 6.1 Add backend and frontend multi-stage Dockerfiles plus Nginx proxy/SPA config. Verify: both images built in Docker without host Go/npm; frontend `.dockerignore` prevents Windows dependencies entering Linux builds.
- [x] 6.2 Add Compose `web`, `api`, and healthy MySQL 8 `db`, exposing only web and defining DB/upload volumes. Verify: `docker compose config --quiet` passed; `ps` showed only web published to host and both named volumes were created.
- [x] 6.3 Verify fresh Compose startup and health. Verify: `docker compose up --build -d`, `docker compose ps`, and proxied `GET /health` all succeeded.

## 7. Verification and handoff

- [x] 7.1 Run the required positive and failure-path acceptance script and preserve results. Verify: 13 PASS lines, zero FAIL, including rejection and success count invariants.
- [x] 7.2 Verify persistence across `docker compose down` and `docker compose up -d`. Verify: material 4, knowledge row, stored filename, file SHA-256, and authenticated download all survived.
- [x] 7.3 Write README with architecture, prerequisites, setup, accounts, demonstrations, health, stop, and explicit volume cleanup. Verify: followed startup and verification instructions from the fresh Compose environment.
- [x] 7.4 Write `docs/iteration-1-verification.md` with environment facts, evidence, remaining work, and the two-minute demonstration. Verify: updated below with actual Docker, acceptance, and persistence evidence.
- [x] 7.5 Re-run strict OpenSpec validation after implementation. Verify: temporary official OpenSpec CLI exited 0 with `Change 'add-auth-rbac-class-knowledge' is valid`.
