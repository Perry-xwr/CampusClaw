## Purpose

Enforce teacher/student permissions at the API boundary.

## ADDED Requirements

### Requirement: Only teachers can upload materials

The system MUST authorize upload from the server-side session role and MUST NOT trust client-submitted role fields or UI visibility.

#### Scenario: Student upload is rejected without side effects

- **GIVEN** a valid student session
- **WHEN** the student posts a valid file to `POST /api/materials`
- **THEN** the API MUST return HTTP 403
- **AND** `materials` count MUST NOT increase
- **AND** `knowledge_entries` count MUST NOT increase
- **AND** upload file count MUST NOT increase

#### Scenario: Student forges teacher role

- **GIVEN** a valid student session
- **WHEN** the student submits `role=teacher` with an upload
- **THEN** the API MUST still return HTTP 403 with no database or filesystem side effects

### Requirement: Students have read-only material access

Students MUST be able to list, view, and download materials in their class but MUST NOT receive an upload control in the UI.

#### Scenario: Student views own-class materials

- **GIVEN** student A1 is logged in
- **WHEN** the material page loads
- **THEN** A-class materials MUST be displayed
- **AND** the upload control MUST NOT be displayed

