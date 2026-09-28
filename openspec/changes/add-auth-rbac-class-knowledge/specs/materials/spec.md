## Purpose

Store and serve class-scoped teaching materials with strict text upload validation.

## ADDED Requirements

### Requirement: Teachers can upload supported UTF-8 text materials

The system MUST accept only non-empty UTF-8 `.txt` and `.md` files within the configured size limit from teachers and MUST save them under a generated filename.

#### Scenario: A-class teacher uploads Markdown

- **GIVEN** teacher A is logged in
- **WHEN** a valid non-empty UTF-8 `.md` file is uploaded
- **THEN** the file MUST be saved in persistent upload storage
- **AND** one `materials` row MUST be created with class A and teacher A
- **AND** the response MUST be HTTP 201 with the new material ID

#### Scenario: Submitted class is ignored

- **GIVEN** teacher A is logged in
- **WHEN** a valid upload includes `class_id=B`
- **THEN** the stored material MUST belong to class A

### Requirement: Invalid uploads leave no residue

Rejected or failed uploads MUST leave material count, knowledge entry count, and upload file count unchanged.

#### Scenario: Unsupported extension

- **WHEN** a teacher uploads an `.exe` file
- **THEN** the API MUST return HTTP 400 with no residual row or file

#### Scenario: Empty file

- **WHEN** a teacher uploads an empty `.txt` or `.md` file
- **THEN** the API MUST return HTTP 400 with no residual row or file

#### Scenario: Invalid UTF-8

- **WHEN** a teacher uploads a supported extension containing invalid UTF-8 bytes
- **THEN** the API MUST return HTTP 400 with no residual row or file

#### Scenario: File exceeds limit

- **WHEN** a teacher uploads a request exceeding the documented maximum size
- **THEN** the API MUST return HTTP 413 with no residual row or file

### Requirement: Class users can read stored materials

Authenticated users MUST be able to list, inspect metadata, and download source files within their class.

#### Scenario: Own-class file download

- **GIVEN** an authenticated user and a material in the same class
- **WHEN** `GET /api/materials/:id/file` is requested
- **THEN** the API MUST return the saved content with a safe attachment filename

