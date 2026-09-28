## Purpose

Create inspectable knowledge source records from uploaded teaching text and deliver the system reproducibly.

## ADDED Requirements

### Requirement: Successful upload creates linked knowledge content

Every successful material upload MUST create exactly one linked `knowledge_entries` row containing the decoded source text and the same authoritative class ID.

#### Scenario: Material is ingested

- **GIVEN** teacher A uploads a valid text material
- **WHEN** ingestion completes
- **THEN** one knowledge entry MUST reference the new material ID
- **AND** its class MUST be A and its content MUST equal the uploaded UTF-8 text

### Requirement: Ingestion is atomic across visible state

An ingestion error MUST roll back its database transaction and remove request-created files.

#### Scenario: Database insertion fails

- **WHEN** material metadata or knowledge insertion fails
- **THEN** no partial `materials` or `knowledge_entries` row may remain
- **AND** no orphaned uploaded file may remain

### Requirement: Seed data supports two-class verification

Initialization MUST create classes A and B, teacher A, student A1, student B1, and a distinguishable material plus knowledge entry for each class. Passwords MUST be bcrypt hashes.

#### Scenario: Fresh database is seeded

- **WHEN** the API initializes a fresh MySQL database
- **THEN** both classes and all three users MUST exist
- **AND** A and B material titles MUST be distinguishable
- **AND** no stored password field may equal a documented plaintext test password

### Requirement: Docker Compose is the reproducible launch path

The repository MUST define `web`, `api`, and MySQL 8 `db` services. Only `web` MUST publish a host port; database and uploaded files MUST use named volumes.

#### Scenario: Fresh Compose environment starts

- **GIVEN** Docker Desktop and Docker Compose and a copied `.env`
- **WHEN** `docker compose up --build -d` completes
- **THEN** all services MUST become healthy/running
- **AND** unauthenticated `GET /health` through `web` MUST return HTTP 200 with `status: ok`

#### Scenario: Data persists across restart

- **GIVEN** a successfully uploaded material
- **WHEN** `docker compose down` followed by `docker compose up -d` is run without deleting volumes
- **THEN** the material row, knowledge entry, and downloadable file MUST still exist

