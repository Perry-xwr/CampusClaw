## Purpose

Authenticate seeded CampusClaw users with server-side sessions and protect business APIs.

## ADDED Requirements

### Requirement: Password login establishes a server-side session

The system MUST validate credentials against a bcrypt hash and MUST create an opaque server-side session containing authoritative user identity through its user relationship.

#### Scenario: Correct teacher credentials

- **GIVEN** the seeded teacher A account
- **WHEN** the correct username and password are submitted to `POST /api/login`
- **THEN** the response MUST succeed and set an HttpOnly session cookie
- **AND** `GET /api/me` with that cookie MUST identify the teacher, role, and class

#### Scenario: Wrong password

- **GIVEN** an existing username and an incorrect password
- **WHEN** login is attempted
- **THEN** the API MUST return HTTP 401
- **AND** MUST NOT establish a valid session or expose a password hash

### Requirement: Protected APIs reject unauthenticated requests

All business APIs except login and health MUST require a valid, unexpired session.

#### Scenario: Missing session

- **GIVEN** an unauthenticated client
- **WHEN** it requests `GET /api/materials`
- **THEN** the API MUST return HTTP 401
- **AND** the body MUST NOT contain material data

### Requirement: Logout revokes the current session

Logout MUST delete the server-side session and expire its cookie.

#### Scenario: Access after logout

- **GIVEN** a logged-in user
- **WHEN** the user calls `POST /api/logout` and then calls `GET /api/me`
- **THEN** logout MUST succeed
- **AND** the later request MUST return HTTP 401

