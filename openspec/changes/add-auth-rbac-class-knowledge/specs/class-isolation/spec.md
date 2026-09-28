## Purpose

Make the authenticated user's class an enforced data boundary.

## ADDED Requirements

### Requirement: Material lists are scoped to the session class

The API MUST derive class scope from the authenticated user and MUST ignore any client-supplied class selector.

#### Scenario: A-class user lists materials

- **GIVEN** an A-class user session
- **WHEN** `GET /api/materials` is requested
- **THEN** every returned material MUST belong to class A
- **AND** no distinguishable B-class seeded title may appear

#### Scenario: A-class user requests class B query

- **GIVEN** an A-class user session
- **WHEN** `GET /api/materials?class_id=B` is requested
- **THEN** the result MUST still contain only class A materials

### Requirement: Object access uses ID and session class

Detail and file queries MUST constrain both object ID and the session-derived class. Cross-class and nonexistent IDs MUST have the same observable response.

#### Scenario: A-class user accesses B-class material ID

- **GIVEN** an A-class user session and a B-class material ID
- **WHEN** detail or file access is requested for that ID
- **THEN** the API MUST return HTTP 404
- **AND** the response MUST NOT disclose the B-class title, content, filename, path, or class

