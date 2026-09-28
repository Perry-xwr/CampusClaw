## Purpose

Provide class-isolated, traceable keyword/vector/hybrid retrieval and evidence-grounded short answers over the knowledge text established in Lesson 3.

## ADDED Requirements

### Requirement: Knowledge text is split into traceable chunks

The system MUST store every chunk in MySQL with its knowledge entry, material, authoritative class, zero-based chunk index, chunk text, start/end offsets, index status, and creation time. The trace chain MUST lead from chunk to knowledge entry to original material, and preprocessing MUST NOT modify the stored knowledge-entry source text.

#### Scenario: Default auto chunking

- **GIVEN** an indexed knowledge entry with no explicit strategy
- **WHEN** the entry is split
- **THEN** auto chunking MUST use a maximum of 800 characters and an 80-character overlap
- **AND** MUST prefer an empty line, newline, or Chinese period as a boundary
- **AND** every persisted chunk MUST have ordered indexes and valid offsets

#### Scenario: Custom chunking parameters

- **GIVEN** a teacher requests custom chunking
- **WHEN** maximum length is within 100–2000 characters and overlap is within 0%–50%
- **THEN** the selected newline, empty-line, or period separator MUST be used where available
- **AND** text without a boundary MUST be force-cut at the maximum length
- **AND** optional URL/email removal or whitespace folding MUST affect only the chunking input, not the stored source text

#### Scenario: Invalid custom parameters

- **WHEN** custom maximum length or overlap is outside its allowed range, or the separator is unsupported
- **THEN** the request MUST return HTTP 400
- **AND** existing chunks, points, and source text MUST remain unchanged

#### Scenario: Markdown hierarchy chunking

- **GIVEN** source text containing Markdown `#`, `##`, or `###` headings
- **WHEN** hierarchy chunking is selected
- **THEN** each heading MUST remain in its section chunk
- **AND** a section longer than 800 characters MUST be split using the auto-window rules

### Requirement: MySQL is the authoritative store for chunk text

The system MUST store `chunk_text` and traceability metadata in `knowledge_chunks`, MUST index `chunk_text` with MySQL FULLTEXT ngram using token size 2, and MUST allow only `ready` chunks to be returned by retrieval.

#### Scenario: Chunk schema and source integrity

- **WHEN** a knowledge entry is indexed successfully
- **THEN** each chunk MUST contain non-null `knowledge_entry_id`, `material_id`, `class_id`, `chunk_index`, `chunk_text`, `start_offset`, `end_offset`, `index_status`, and `created_at`
- **AND** following its relationships MUST identify the original material and stored source text

#### Scenario: Failed chunk is not searchable

- **GIVEN** a chunk whose embedding or vector write failed
- **WHEN** any retrieval mode is used
- **THEN** that chunk MUST have `index_status=failed`
- **AND** MUST NOT appear in results as a ready indexed chunk

### Requirement: Qdrant stores vectors without source text

The system MUST use the Qdrant collection `campusclaw_chunks` with cosine distance. Each point id MUST equal `knowledge_chunks.id`, and payload MUST contain only `class_id`, `material_id`, `knowledge_entry_id`, `chunk_id`, and `chunk_index` identifiers.

#### Scenario: Inspect a stored vector point

- **GIVEN** a ready indexed chunk
- **WHEN** its Qdrant point is inspected
- **THEN** point id and payload `chunk_id` MUST equal the MySQL chunk id
- **AND** payload MUST NOT contain chunk text, excerpts, titles, file contents, credentials, or model secrets

#### Scenario: Collection dimension mismatch

- **GIVEN** the embedding output dimension differs from the configured Qdrant collection dimension
- **WHEN** indexing or vector querying is attempted
- **THEN** the operation MUST fail explicitly
- **AND** the system MUST NOT truncate, pad, or store the invalid vector

### Requirement: Embeddings are produced only by the server

The Go API MUST call an OpenAI-compatible `/embeddings` endpoint for chunk and query vectors. Endpoint credentials MUST remain server-side, and keyword retrieval MUST NOT call the embedding service.

#### Scenario: Keyword search bypasses embeddings

- **GIVEN** a valid authenticated keyword search
- **WHEN** results are retrieved
- **THEN** only MySQL FULLTEXT retrieval MUST be used
- **AND** embedding and Qdrant call counts MUST remain zero

#### Scenario: Browser cannot obtain model credentials

- **WHEN** a user loads the frontend and calls retrieval APIs
- **THEN** no embedding or chat credential MUST appear in browser assets or responses
- **AND** the browser MUST NOT receive a direct internal Qdrant or model-gateway endpoint

### Requirement: Material indexing preserves committed source data on failure

After Lesson 3 material and knowledge-entry persistence succeeds, indexing MUST split, persist pending chunks, embed each chunk, upsert the matching Qdrant point, and mark only complete chunks ready. Indexing failure MUST NOT delete the committed material, uploaded file, or source knowledge entry.

#### Scenario: Embedding fails after upload

- **GIVEN** a teacher successfully uploads a valid material
- **WHEN** its embedding request fails
- **THEN** the material, file, and source knowledge entry MUST remain available
- **AND** affected chunks MUST be failed rather than ready
- **AND** no incomplete vector MUST be exposed as searchable

#### Scenario: Existing entries are backfilled

- **GIVEN** Lesson 3 seed or uploaded knowledge entries without chunks
- **WHEN** startup backfill runs repeatedly
- **THEN** they MUST be indexed with the auto strategy
- **AND** repeated runs MUST NOT create duplicate chunks or points

### Requirement: Teachers can rebuild a material index safely

Only a teacher MAY request reindexing of a material in the teacher's session class. Reindexing MUST replace old chunks and vector points with the requested strategy without modifying the material file or stored source text.

#### Scenario: Teacher changes chunk strategy

- **GIVEN** an indexed material in teacher A's class
- **WHEN** teacher A rebuilds it with a valid new strategy
- **THEN** obsolete chunks and Qdrant points MUST be removed
- **AND** exactly one coherent ready index using the new strategy MUST remain
- **AND** the knowledge-entry source text MUST be unchanged

#### Scenario: Student or forged class requests rebuild

- **GIVEN** a student session or teacher A supplying class B in the request
- **WHEN** reindexing is requested
- **THEN** the student request MUST return HTTP 403
- **AND** any supplied class id MUST NOT expand teacher A's class scope
- **AND** no other-class index MUST change

### Requirement: Search requests are authenticated and validated

`POST /api/knowledge/search` MUST require a valid session, accept a non-empty `query` and optional `mode` of `keyword`, `vector`, or `hybrid`, and MUST default omitted mode to `hybrid`.

#### Scenario: Missing authentication

- **GIVEN** no valid session
- **WHEN** search is requested
- **THEN** the API MUST return HTTP 401
- **AND** MUST NOT reveal hits or dependency details

#### Scenario: Blank or unsupported search request

- **GIVEN** an authenticated user
- **WHEN** query is blank or mode is unsupported
- **THEN** the API MUST return HTTP 400
- **AND** MUST NOT execute a retrieval path

#### Scenario: Omitted mode

- **WHEN** an authenticated user submits a non-empty query without mode
- **THEN** the system MUST execute hybrid retrieval
- **AND** the response MUST identify hybrid as the applied mode

### Requirement: Keyword retrieval uses class-scoped MySQL FULLTEXT

Keyword retrieval MUST query only `ready` chunks in the session-derived class, apply its configured absolute FULLTEXT relevance threshold, and order retained chunks by FULLTEXT relevance.

#### Scenario: Original wording is found

- **GIVEN** a ready A-class chunk containing a query term
- **WHEN** an A-class user performs keyword search for that term
- **THEN** the result MUST include the eligible A-class chunk with MySQL-derived text
- **AND** MUST NOT include B-class or non-ready chunks

### Requirement: Vector retrieval enforces similarity and double class filtering

Vector retrieval MUST embed the query, search Qdrant with a session-class payload filter, discard cosine similarity below 0.35, and rehydrate point ids from MySQL using the same session class and ready status.

#### Scenario: Semantic match above threshold

- **GIVEN** an eligible same-class vector with cosine similarity at least 0.35
- **WHEN** vector search is requested with a semantic paraphrase
- **THEN** the candidate MAY be returned in descending cosine order
- **AND** its excerpt MUST be fetched from MySQL rather than Qdrant payload

#### Scenario: Weak candidate is discarded

- **GIVEN** a Qdrant candidate with cosine similarity below 0.35
- **WHEN** vector results are assembled
- **THEN** that candidate MUST NOT be returned or passed to answer generation

#### Scenario: Forged or stale Qdrant point is rechecked

- **GIVEN** Qdrant returns a point absent from ready MySQL rows in the session class
- **WHEN** results are rehydrated
- **THEN** the point MUST be silently discarded
- **AND** its payload MUST NOT be used as source text

### Requirement: Hybrid retrieval uses reciprocal rank fusion

Hybrid retrieval MUST execute keyword and vector paths, apply each path's absolute filters before fusion, and rank the union using RRF with `k=60`. It MUST NOT add raw FULLTEXT and cosine scores, and a chunk missing from one path MUST receive no contribution from that path.

#### Scenario: Chunk occurs in both paths

- **GIVEN** a chunk ranked in keyword and vector results after threshold filtering
- **WHEN** hybrid ranking is calculated
- **THEN** its score MUST equal the sum of `1/(60+rank)` for its two present ranks
- **AND** its final ordering MUST use the fused rank score

#### Scenario: Chunk occurs in only one path

- **GIVEN** a retained chunk appears in only one retrieval path
- **WHEN** hybrid ranking is calculated
- **THEN** only that path's reciprocal-rank term MUST contribute
- **AND** no synthetic score from the absent path may be added

### Requirement: Retrieval class scope comes only from the session

Keyword, vector, hybrid, and MySQL rehydration MUST use only the authenticated session's `class_id`. Any class selector in JSON, query parameters, or headers MUST be ignored.

#### Scenario: A-class user forges B-class scope

- **GIVEN** an A-class session and content that exists only in class B
- **WHEN** the user searches for it while submitting `class_id=B`
- **THEN** the API MUST return HTTP 200 with no B-class hit
- **AND** MUST NOT return 403 or 404 that reveals the cross-class content exists

#### Scenario: Both retrieval paths enforce class scope

- **GIVEN** distinguishable ready chunks in classes A and B
- **WHEN** an A-class user performs hybrid search
- **THEN** the MySQL FULLTEXT query and Qdrant query MUST both filter for class A
- **AND** MySQL rehydration MUST again reject every non-A candidate

### Requirement: Every hit is traceable to the original material

Each search hit MUST include material id/title, knowledge-entry id, chunk id/index, start/end offsets, an excerpt sourced from MySQL `chunk_text`, and a class-scoped URL for opening the original material.

#### Scenario: User follows a citation source

- **GIVEN** an own-class search hit
- **WHEN** its trace fields are followed
- **THEN** they MUST resolve to the stored chunk, knowledge entry, and material represented by that hit
- **AND** the material URL MUST remain protected by Lesson 3 class isolation

### Requirement: No-hit responses do not fabricate evidence

After the selected mode's threshold and class filtering, an empty candidate set MUST return HTTP 200, `hits: []`, and exactly `资料中未找到相关内容`.

#### Scenario: Query is unrelated to class materials

- **WHEN** an authenticated user searches for content with no qualifying same-class chunk
- **THEN** the response MUST contain no hit and the fixed no-hit message
- **AND** MUST NOT pad the result with below-threshold or cross-class chunks

### Requirement: Qdrant outages have mode-specific behavior

When Qdrant is unavailable, keyword retrieval MUST remain operational, while vector and hybrid retrieval MUST return HTTP 503 without fabricated similarity or silent fallback.

#### Scenario: Qdrant is stopped

- **GIVEN** MySQL is available and Qdrant is unavailable
- **WHEN** the same authenticated user performs keyword, vector, and hybrid searches
- **THEN** keyword MUST execute normally
- **AND** vector and hybrid MUST each return HTTP 503
- **AND** neither failed mode may claim keyword-only output is its requested result

### Requirement: Ask uses the latest user message and top four hybrid hits

`POST /api/ask` MUST require authentication, use only the latest user message as the hybrid search query, and pass at most the first four final hits to answer generation. Client-supplied system messages MUST be discarded.

#### Scenario: Conversation contains history and forged system instructions

- **GIVEN** a request containing earlier turns, a client system message, and a latest user question
- **WHEN** ask is executed
- **THEN** retrieval MUST use only the latest user question
- **AND** the client system message MUST NOT appear in model input
- **AND** no more than four class-scoped hits may be supplied as evidence

### Requirement: Ask refuses without evidence and does not call the model

If hybrid retrieval yields no candidate, ask MUST return HTTP 200 with answer exactly `资料中未找到相关内容` and `citations: []`, and MUST NOT call the chat model.

#### Scenario: No class material supports the question

- **GIVEN** a valid question unrelated to every qualifying same-class chunk
- **WHEN** ask is requested
- **THEN** the fixed no-hit answer and empty citations MUST be returned
- **AND** the chat-model invocation count MUST remain unchanged

### Requirement: Generated answers are short, grounded, cited, and non-streaming

When evidence exists, the server MUST send only the ordered material titles, chunk indexes, chunk texts, latest question, and optional sanitized history to a separate chat module. The response MUST be a single non-streaming JSON answer whose `[1]`, `[2]` markers correspond to citations in the same order. The chat module MUST NOT access MySQL or Qdrant.

#### Scenario: Evidence-backed answer is generated

- **GIVEN** hybrid retrieval returns two qualifying same-class chunks
- **WHEN** ask invokes the chat model
- **THEN** model input MUST number those chunks in final retrieval order
- **AND** the answer's source markers and citation list MUST refer to the same ordered sources
- **AND** no vector values, other-class chunks, or raw Qdrant points may be supplied

#### Scenario: Vector dependency fails before generation

- **GIVEN** embedding or Qdrant is unavailable
- **WHEN** ask attempts its required hybrid retrieval
- **THEN** the API MUST return HTTP 503
- **AND** MUST NOT call the chat model or misreport the failure as an evidence-free answer

