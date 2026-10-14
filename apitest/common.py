from __future__ import annotations

import json
import os
import subprocess
import urllib.error
import urllib.request
import uuid
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


@dataclass
class Context:
    base_url: str
    token_a_student: str = ""
    token_a_teacher: str = ""
    token_b_student: str = ""
    prompt_id: int = 0
    skill_id: int = 0
    session_id: int = 0
    message_id: int = 0
    values: dict = field(default_factory=dict)
    run_id: str = field(default_factory=lambda: datetime.now().strftime("%Y%m%d%H%M%S") + "-" + uuid.uuid4().hex[:8])
    created_prompts: list = field(default_factory=list)
    created_skills: list = field(default_factory=list)
    created_sessions: list = field(default_factory=list)
    issued_tokens: list = field(default_factory=list)

    def request(self, method: str, path: str, token: str = "", payload=None):
        headers = {"Accept": "application/json"}
        body = None
        if token:
            headers["Authorization"] = f"Bearer {token}"
        if payload is not None:
            body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
            headers["Content-Type"] = "application/json; charset=utf-8"
        request = urllib.request.Request(self.base_url + path, data=body, headers=headers, method=method)
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                return response.status, response.headers, response.read()
        except urllib.error.HTTPError as error:
            return error.code, error.headers, error.read()

    def json(self, method: str, path: str, token: str = "", payload=None):
        status, headers, body = self.request(method, path, token, payload)
        try:
            return status, json.loads(body.decode("utf-8")) if body else None
        except (UnicodeDecodeError, json.JSONDecodeError) as error:
            raise AssertionError(f"{method} {path} returned invalid JSON: {body[:200]!r}") from error

    def db_scalar(self, sql: str) -> str:
        env = os.environ.copy()
        env.update(read_dotenv(ROOT / ".env"))
        user = env.get("MYSQL_USER", "campusclaw")
        database = env.get("MYSQL_DATABASE", "campusclaw")
        env["MYSQL_PWD"] = env.get("MYSQL_PASSWORD", "")
        command = ["docker", "compose", "exec", "-T", "-e", "MYSQL_PWD", "db", "mysql", "-N", "-B", f"-u{user}", database, "-e", sql]
        # Docker's Windows CLI forwards UTF-8 text from the Linux MySQL container;
        # use that encoding explicitly instead of the host's legacy GBK code page.
        result = subprocess.run(command, cwd=ROOT, env=env, text=True, encoding="utf-8", errors="replace", capture_output=True, timeout=30)
        if result.returncode:
            raise RuntimeError("MySQL assertion failed; confirm Docker Compose services are running")
        return result.stdout.strip().splitlines()[-1] if result.stdout.strip() else ""

    def db_exec(self, sql: str) -> None:
        env = os.environ.copy()
        env.update(read_dotenv(ROOT / ".env"))
        env["MYSQL_PWD"] = env.get("MYSQL_PASSWORD", "")
        command = ["docker", "compose", "exec", "-T", "-e", "MYSQL_PWD", "db", "mysql", "-N", "-B", f"-u{env.get('MYSQL_USER', 'campusclaw')}", env.get("MYSQL_DATABASE", "campusclaw"), "-e", sql]
        result = subprocess.run(command, cwd=ROOT, env=env, text=True, encoding="utf-8", errors="replace", capture_output=True, timeout=30)
        if result.returncode:
            raise RuntimeError("MySQL cleanup failed; newly created test rows were retained where possible")

    def cleanup_created_data(self) -> list[str]:
        """Delete only IDs returned to this process by this run's create requests."""
        errors = []
        for token in self.issued_tokens:
            try:
                status, _, _ = self.request("POST", "/api/logout", token=token)
                if status != 204:
                    errors.append("login session could not be revoked through logout API")
            except Exception:
                errors.append("login session could not be revoked through logout API")
        for session_id in reversed(self.created_sessions):
            try:
                self.db_exec(f"DELETE FROM chat_sessions WHERE id={int(session_id)} AND user_id=2 AND class_id=1")
                if self.db_scalar(f"SELECT COUNT(*) FROM chat_sessions WHERE id={int(session_id)} AND user_id=2 AND class_id=1") != "0":
                    errors.append(f"session {session_id} remains; retained for review")
            except Exception as error:
                errors.append(f"session {session_id}: {error}")
        for skill_id, slug in reversed(self.created_skills):
            try:
                safe_slug = slug.replace("'", "''")
                self.db_exec(f"DELETE FROM teacher_skills WHERE id={int(skill_id)} AND class_id=1 AND created_by=1 AND slug LIKE 'apitest-{self.run_id}%' AND slug='{safe_slug}'")
                if self.db_scalar(f"SELECT COUNT(*) FROM teacher_skills WHERE id={int(skill_id)} AND class_id=1 AND created_by=1") != "0":
                    errors.append(f"skill {skill_id} remains; retained for review")
            except Exception as error:
                errors.append(f"skill {skill_id}: {error}")
        for prompt_id, name in reversed(self.created_prompts):
            try:
                safe_name = name.replace("'", "''")
                self.db_exec(f"DELETE FROM teacher_prompts WHERE id={int(prompt_id)} AND class_id=1 AND created_by=1 AND name LIKE 'APITEST-{self.run_id}%' AND name='{safe_name}'")
                if self.db_scalar(f"SELECT COUNT(*) FROM teacher_prompts WHERE id={int(prompt_id)} AND class_id=1 AND created_by=1") != "0":
                    errors.append(f"prompt {prompt_id} remains; retained for review")
            except Exception as error:
                errors.append(f"prompt {prompt_id}: {error}")
        return errors

    def mock_stats(self) -> dict:
        command = ["docker", "compose", "exec", "-T", "model-mock", "python", "-c", "import urllib.request; print(urllib.request.urlopen('http://127.0.0.1:8081/stats', timeout=5).read().decode())"]
        result = subprocess.run(command, cwd=ROOT, text=True, encoding="utf-8", errors="replace", capture_output=True, timeout=15)
        if result.returncode:
            raise RuntimeError("Could not inspect model-mock request statistics")
        import json
        return json.loads(result.stdout)


def read_dotenv(path: Path) -> dict[str, str]:
    values = {}
    if not path.exists():
        return values
    for line in path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#") or "=" not in stripped:
            continue
        key, value = stripped.split("=", 1)
        values[key.strip()] = value.strip().strip('"').strip("'")
    return values


def check_status(actual: int, expected: int, response=None):
    if actual != expected:
        raise AssertionError(f"expected HTTP {expected}, got {actual}: {response!r}")
