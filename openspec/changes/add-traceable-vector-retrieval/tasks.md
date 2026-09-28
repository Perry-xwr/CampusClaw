## 1. Schema and deployment

- [ ] 1.1 Add an idempotent MySQL migration for `knowledge_chunks`, foreign keys, unique/source indexes, `index_status`, and ngram FULLTEXT on `chunk_text`. Verify: migrate a fresh and an existing Iteration 1 database, inspect columns/constraints/indexes, and confirm `ngram_token_size=2`.
- [ ] 1.2 Add an internal Qdrant service, health check, and persistent named volume to Compose without publishing its port. Verify: `docker compose config`, service health, and host port inspection.
- [ ] 1.3 Add documented server-only embedding, chat, Qdrant, dimension, timeout, and threshold configuration. Verify: configuration validation rejects missing/invalid dimensions and no key appears in the frontend bundle or API response.
- [ ] 1.4 Implement idempotent creation/validation of the `campusclaw_chunks` cosine collection. Verify: repeated startup succeeds and a dimension/distance mismatch fails explicitly.

## 2. Chunking and indexing

- [ ] 2.1 Implement the auto chunker with maximum 800 characters, 80-character overlap, and preferred paragraph/newline/period boundaries. Verify: boundary, overlap, long-unbroken-text, and Unicode tests.
- [ ] 2.2 Implement the custom chunker with allowed separators, length 100–2000, overlap 0%–50%, forced cuts, and optional URL/email removal and whitespace folding. Verify: parameter validation and preprocessing/offset tests.
- [ ] 2.3 Implement Markdown hierarchy chunking for `#`/`##`/`###`, retaining headings and applying auto windows to oversized sections. Verify: heading nesting, preamble, oversized section, and no-heading cases.
- [ ] 2.4 Persist deterministic chunk order, text, offsets, source IDs, class ID, and pending/ready/failed state without changing `knowledge_entries.content`. Verify: database assertions against representative source text.
- [ ] 2.5 Implement the OpenAI-compatible embeddings client with timeouts, bounded responses, dimension checks, and secret-safe errors. Verify: mock success, HTTP failure, malformed response, and wrong-dimension tests.
- [ ] 2.6 Implement Qdrant point upsert/delete with point id equal to chunk id and identifier-only payload. Verify: inspect stored points and confirm no title or chunk text is present.
- [ ] 2.7 Connect successful material ingestion to the default auto indexing workflow while preserving committed material/original text on indexing failure. Verify: success yields ready chunks; forced embedding failure yields retained material and failed chunks without incomplete ready vectors.
- [ ] 2.8 Backfill seed and pre-existing knowledge entries with the auto strategy idempotently. Verify: repeated startup creates neither duplicate chunks nor duplicate points.
- [ ] 2.9 Add a teacher-only, class-scoped explicit reindex operation for auto/custom/hierarchy. Verify: students receive 403, forged class is ignored, and the requested strategy replaces only the teacher's class material index.
- [ ] 2.10 Implement replacement/compensation cleanup for old chunks, Qdrant points, partial failures, and orphan points. Verify: fault injection never exposes mixed old/new ready indexes and retry converges to one complete index.

## 3. Retrieval

- [ ] 3.1 Implement keyword retrieval using MySQL FULLTEXT with session class, ready status, an absolute score threshold, and stable ranking. Verify: original Chinese terms match and the embedding/Qdrant mocks receive zero calls.
- [ ] 3.2 Implement query embedding and Qdrant vector search with session-class payload filtering and cosine threshold 0.35. Verify: below-threshold points are discarded and only the session class is requested.
- [ ] 3.3 Rehydrate vector candidates from MySQL by chunk id plus session class and ready status. Verify: forged/cross-class/stale Qdrant points are silently excluded and excerpts equal MySQL `chunk_text`.
- [ ] 3.4 Implement hybrid retrieval with per-path absolute filtering and RRF k=60. Verify: hand-calculated rankings, missing-path contributions, stable ties, and proof that raw scores are not added.
- [ ] 3.5 Add authenticated `POST /api/knowledge/search`, default hybrid mode, request validation, result limits, and fixed no-hit semantics. Verify: unauthenticated 401, blank/invalid requests 400, and unrelated queries return 200 with empty hits and the fixed message.
- [ ] 3.6 Return traceable source fields for every hit, including material/entry/chunk IDs, title, chunk index, offset interval, MySQL excerpt, and material URL. Verify: each response can be followed to the stored chunk and class-scoped original material.
- [ ] 3.7 Enforce class scope from the session across keyword, Qdrant, and MySQL rehydration, ignoring body/query/header class IDs. Verify: A-class searches for B-only content, including forged `class_id=B`, return 200 with no B data.
- [ ] 3.8 Implement dependency-specific failure behavior. Verify: with Qdrant stopped keyword still succeeds, while vector and hybrid return 503 without fabricated scores or silent fallback.

## 4. Evidence-grounded ask

- [ ] 4.1 Implement the OpenAI-compatible non-streaming chat client as a separate server-only module with timeout, bounded response, and secret-safe errors. Verify: mock success/failure and confirm it has no DB or Qdrant access.
- [ ] 4.2 Add authenticated `POST /api/ask` and select only the latest user message as the hybrid retrieval query. Verify: older turns do not alter retrieval and missing/blank latest questions return 400.
- [ ] 4.3 Discard all client-supplied system messages and assemble a server-owned evidence prompt from at most the top four class-scoped hits. Verify: captured model input contains titles, indexes, chunk text, and question but no vectors, foreign-class text, or client system prompt.
- [ ] 4.4 Short-circuit no-hit questions with `资料中未找到相关内容` and empty citations. Verify: response is 200 and the chat-model call count remains zero.
- [ ] 4.5 Generate short grounded answers with `[1]`, `[2]` markers and ordered citations matching the retrieval order. Verify: marker/citation mapping and source traceability for one-hit and multi-hit cases.
- [ ] 4.6 Return 503 without invoking chat when hybrid retrieval's embedding or Qdrant dependency is unavailable. Verify: injected failures produce no model call and are not mislabeled as no-hit.

## 5. Verification and handoff

- [ ] 5.1 Implement a minimal authenticated search UI for keyword, vector, and default hybrid modes with loading, no-hit, and 503 states. Verify: both teacher and student can search only their class and the selected/default mode is visible.
- [ ] 5.2 Implement source rendering and navigation from each hit/citation to the protected original material. Verify: title, chunk index, offsets, and excerpt render from the API and cross-class material URLs still return 404.
- [ ] 5.3 Add Go unit/integration tests for all chunk strategies, index state transitions, dimension validation, keyword/vector/hybrid ranking, class isolation, and failure handling. Verify: `go test ./...` exits 0.
- [ ] 5.4 Extend the acceptance script for three retrieval modes, default hybrid, 0.35 filtering, RRF k=60, trace fields, forged class IDs, B-only queries, and Qdrant outage behavior. Verify: every new PASS assertion runs against Compose.
- [ ] 5.5 Add ask acceptance tests for latest-message retrieval, discarded system injection, top-four evidence, ordered citations, no-hit fixed text, zero model calls, and non-streaming JSON. Verify: all positive and negative assertions pass with deterministic mock gateways.
- [ ] 5.6 Verify persistence and rebuild behavior across `docker compose down` then `up -d` without `-v`. Verify: MySQL chunks, Qdrant points, source files, search hits, and citations remain valid.
- [ ] 5.7 Update README and lesson-4 verification notes with architecture, configuration, API examples, failure semantics, and a short classroom demo. Verify: a fresh operator can reproduce keyword/vector/hybrid search and grounded/no-hit ask behavior.
- [ ] 5.8 Run final security checks for session-only class scope, unexposed Qdrant/model endpoints, payload contents, and secret leakage. Verify: automated probes cannot retrieve another class or reach internal services from the host/browser.
- [ ] 5.9 Run strict OpenSpec validation after all Apply work. Verify: `openspec validate add-traceable-vector-retrieval --strict` exits 0.
