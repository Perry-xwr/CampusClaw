# CampusClaw API acceptance tests

These scripts exercise the running CampusClaw HTTP API, including authentication, class isolation, knowledge retrieval, teacher-managed prompts and skills, and persisted streaming conversations. They use only the Python standard library.

## Requirements

- Python 3.10 or newer.
- CampusClaw Compose services running and reachable (default `http://localhost:8080`).
- Docker Compose CLI access from the same shell, because database assertions and model-mock statistics are read through the running containers.
- Seeded accounts and the default `guided-solution` skill, plus the lesson's sample knowledge material used by the retrieval/SSE checks.

## Run

From the repository root:

```powershell
python apitest/run_all.py
```

The base URL defaults to `http://localhost:8080`. `APITEST_BASE_URL` in the project `.env` can override it. The runner exits with code 0 only when all API assertions and current-run cleanup succeed; otherwise it exits with code 1. Tokens created by the login test are revoked through the logout API at the end of the run.

## Reports

HTML reports are written to `apitest/reports/`. This directory is ignored by Git. Reports contain test names, assertion summaries, timings, and the target URL; they do not contain Authorization headers, returned tokens, passwords, or API keys.

## Test data and cleanup

Every run gets a timestamp plus random run identifier. Prompt names, skill slugs, and session titles created through the API carry that identifier. The runner records only IDs returned by successful create requests in the current process. After all cases, it revokes its own login tokens, deletes those exact current-run sessions first (their messages are removed by the database's `ON DELETE CASCADE` foreign key), then only the exact newly-created skill and prompt rows with the expected class/creator and run identifier. It never truncates tables and never removes the seeded `guided-solution` skill, users, materials, or records from earlier runs.

If a cleanup statement fails, the run is marked failed and the HTML report records the affected row IDs; those rows are retained for review. Existing historical test data is intentionally left untouched.

## Layout

Scripts follow the API URL and method: for example, `api/knowledge/search/post.py` tests `POST /api/knowledge/search`, and `api/sessions/id/messages/post.py` tests `POST /api/sessions/{id}/messages`.
