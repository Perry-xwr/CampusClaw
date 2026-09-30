## Context

This is a greenfield course repository. The Iteration 1 deliverable must be independently reproducible and demonstrably reject vertical and horizontal privilege escalation. The course architecture is Browser → Nginx/React → Go API → MySQL 8, launched with Docker Compose.

## Goals / Non-Goals

**Goals:**

- A small demo that is easy to start, inspect, and verify.
- Authentication, authorization, and class isolation implemented as separate server-side layers.
- Atomic material ingestion with persistent files and database rows.
- Failure responses that do not reveal whether a cross-class object exists.

**Non-Goals:** See `proposal.md`; notably, this iteration contains no AI/RAG, assignment workflow, self-registration, or production orchestration.

## Decisions

### Decision 1: Three-service container architecture

Compose defines `web` (Nginx serving the built React app and proxying `/api` and `/health`), `api` (Go), and `db` (MySQL 8). Only `web` publishes a host port. `db` has no host `3306` mapping. Both the API and database have health checks, and dependency conditions ensure startup ordering.

Host Go and Node installations are unnecessary because multi-stage Dockerfiles perform both builds.

### Decision 2: Server-side opaque sessions transported as Bearer tokens

On successful login, the API creates a cryptographically random opaque token, stores only its SHA-256 hash in `sessions`, and returns the raw token once in the JSON login response. The frontend stores the token in `localStorage` for this course demo and sends it as `Authorization: Bearer <token>` on protected requests. The session row is the server-side authority for `user_id`; each request joins the user row to obtain `role` and `class_id`. Client-submitted `role`, `user_id`, and `class_id` are ignored. This localStorage choice is a demonstration trade-off, not a production security recommendation.

Authentication middleware returns 401 for a missing/malformed Authorization header or an invalid/expired token. Role middleware returns 403 when a valid student invokes a teacher-only operation.

### Decision 3: Passwords use bcrypt

`users.password_hash` stores bcrypt hashes. Login uses constant-purpose bcrypt comparison and returns the same 401 message for unknown users and bad passwords. Seed passwords are hashed, not stored in plaintext in the database.

### Decision 4: Class scope is derived from the session

All list queries include `WHERE class_id = ?` with the authenticated user's class. Detail and file queries include both material ID and session-derived class ID. Any query/body `class_id`, including a forged B-class value, is ignored. Missing and cross-class material IDs return the same 404 response body.

### Decision 5: Relational data model

- `classes(id, name, created_at)`
- `users(id, username UNIQUE, password_hash, role ENUM('teacher','student'), class_id FK, created_at)`
- `sessions(id, token_hash UNIQUE, user_id FK, expires_at, created_at)`
- `materials(id, class_id FK, uploader_id FK, title, original_filename, stored_filename UNIQUE, created_at)`
- `knowledge_entries(id, material_id UNIQUE FK, class_id FK, content MEDIUMTEXT, created_at)`

Foreign keys and indexes cover class and session lookups. Seed data contains classes A/B, teacher A, student A1, student B1, and one distinguishable material per class with a linked knowledge entry.

### Decision 6: Upload validation and atomic ingestion

The API caps request bodies before multipart parsing and returns 413 for an over-limit body. It accepts only `.txt` and `.md` (case-insensitive), requires a non-empty payload, validates UTF-8, normalizes a user-provided title, and generates a random stored filename. It never trusts the original path.

The API writes to a temporary file, starts a transaction, inserts `materials` and `knowledge_entries` using the session's class, renames the temporary file to its final name, and commits. Every error rolls back database work and removes temporary/final files created by that request. A serialized upload critical section avoids commit/file cleanup races in this teaching-sized single-replica deployment.

### Decision 7: Minimal React interface

React + TypeScript + Vite provides login, current-user header, material list, file link, logout, and a teacher-only upload form. UI visibility is convenience only; the API remains authoritative. Nginx serves the SPA and proxies same-origin requests so no CORS configuration is needed.

### Decision 8: Health and persistence

`GET /health` is public and checks MySQL with a short timeout. Success returns HTTP 200 JSON with `status: "ok"`; dependency failure returns 503. Named volumes persist `/var/lib/mysql` and `/data/uploads`. Verification restarts containers without `-v` and confirms the uploaded row and file remain.

## Security boundaries

Authentication establishes identity; authorization permits actions; class isolation filters objects. These controls are independently enforced for every API request. The session TTL is finite, raw tokens are never stored in the database or logged, upload names are randomized, SQL uses bound parameters, and error messages omit cross-class metadata and filesystem paths.

## Risks / Trade-offs

- **DB/file atomicity has no distributed transaction:** staged writes plus rollback cleanup minimize orphaning; the single API replica and upload lock make the chosen ordering deterministic for this iteration.
- **Database-backed sessions add a lookup:** acceptable for a course demo and make revocation/logout explicit.
- **Plain-text formats only:** intentionally avoids complex parsers and keeps knowledge ingestion inspectable.
- **Browser storage of bearer tokens has XSS exposure:** `localStorage` keeps the course demo and refresh flow simple, but production deployment should use a threat-model-appropriate token storage and rotation design.

## Migration Plan

Greenfield initialization uses versioned SQL in `backend/migrations/`. The API applies idempotent schema and seed statements at startup after waiting for MySQL. No existing data migration is required.

## Design review judgment

The design is fit for Apply because every trust boundary is server-side, cross-class existence is concealed with uniform 404 responses, upload failure invariants cover database and filesystem state, and the Compose topology matches the course architecture. The principal residual limitation is that real Compose execution requires Docker Desktop on the host; this does not change the implementation design.
