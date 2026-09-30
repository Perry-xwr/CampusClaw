# Iteration 2 verification

## Scope

Applied `add-traceable-vector-retrieval` on branch `lesson4-apply`. Authentication remains `Authorization: Bearer <random-session-token>` backed by server-side sessions. The implementation uses MySQL for source/chunk text, Qdrant `campusclaw_chunks` for vectors and identifier-only payload, and Go for chunking, indexing, retrieval, RRF, and answer orchestration.

## Reproducible provider

No paid model key was available. Compose therefore used the internal deterministic `model-mock` service, which implements the minimal OpenAI-compatible `/embeddings` and `/chat/completions` endpoints. It is internal-only, fixed-dimension, deterministic, and explicitly not a real language model. Real providers use the same Go clients and are selected with the documented environment variables.

## Commands actually run

```powershell
docker run --rm -v E:\CampusClaw\backend:/app -w /app golang:1.24-alpine go test ./...
docker build -t campusclaw-web-lesson4 ./frontend
docker compose config --quiet
docker compose up --build -d
docker compose ps
powershell -ExecutionPolicy Bypass -File scripts/verify.ps1
openspec validate add-traceable-vector-retrieval --strict
```

The host had no Go/npm commands, so Go tests and the exact frontend `tsc --noEmit && vite build` script ran inside the repository Docker build toolchains.

## Observed evidence

- Existing MySQL volume migrated without deletion; `@@ngram_token_size=2`.
- `knowledge_chunks` contains source FKs, unique `(knowledge_entry_id, chunk_index)`, class/status and material indexes, ngram FULLTEXT, and pending/ready/failed status.
- Qdrant v1.12.6 collection is cosine, dimension 16 in mock mode, and persisted in `qdrant_data`.
- Only web publishes a host port; api, db, qdrant, and model-mock remain internal.
- Lesson 3 regression: 22 PASS / 0 FAIL.
- Lesson 4 acceptance: 22 PASS / 0 FAIL.
- Keyword made zero embedding/chat calls; semantic vector sample exceeded 0.35; hybrid top hit matched hand-calculated `2/(60+1)`.
- A-class forged `class_id=2` and B-only keyword returned HTTP 200 with empty hits and the fixed no-hit message.
- No-hit ask made zero chat calls; evidence ask produced `[1]` aligned to citations, and captured mock input excluded the client system and earlier user message.
- With Qdrant stopped, keyword succeeded while vector, hybrid, and ask returned 503; all recovered after restart.
- With model-mock stopped, upload retained file/material/knowledge entry, created failed chunks, and exposed no ready chunk; teacher reindex converged to ready after recovery.
- `docker compose down` followed by `up -d` without `-v` preserved material, knowledge entry, ready chunks, Qdrant point count, search results, and citations.

## Security checks

Class scope is always derived from the Bearer-backed server session. Qdrant filtering and MySQL rehydration repeat the same class constraint. Qdrant payload inspection is restricted to identifiers and excludes chunk text, title, file content, vectors in API responses, and provider keys. Repository scans found no committed `.env`, API key, session token, or model credential.
