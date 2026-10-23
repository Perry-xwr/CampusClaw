"""Small, dependency-free MCP JSON-RPC demo server for the Lesson 6 lab."""
from __future__ import annotations

import hmac
import json
import os
import re
import threading
import uuid
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

PROTOCOL_VERSION = "2024-11-05"
MAX_BODY = 64 * 1024
TOOLS = [
    {
        "name": "formula_hint",
        "description": "按主题给出公式线索，不代入数值或给出最终答案。",
        "inputSchema": {
            "type": "object",
            "properties": {"topic": {"type": "string", "minLength": 1, "maxLength": 80}},
            "required": ["topic"],
            "additionalProperties": False,
        },
    },
    {
        "name": "unit_convert",
        "description": "在支持的公制长度、质量单位之间换算。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "value": {"type": "number"},
                "from_unit": {"type": "string", "enum": ["m", "cm", "km", "g", "kg"]},
                "to_unit": {"type": "string", "enum": ["m", "cm", "km", "g", "kg"]},
            },
            "required": ["value", "from_unit", "to_unit"],
            "additionalProperties": False,
        },
    },
]
FORMULAS = {
    "speed": "可先回忆速度、路程和时间之间的关系，再判断题目要求哪个量。",
    "density": "可从质量与体积的比值关系入手，先确认单位是否一致。",
    "area": "可先识别图形类型，再回忆对应的面积关系。",
    "force": "可先检查质量与加速度之间的关系以及各物理量单位。",
}
UNIT_GROUPS = {
    "m": ("length", 1.0), "cm": ("length", 0.01), "km": ("length", 1000.0),
    "g": ("mass", 1.0), "kg": ("mass", 1000.0),
}
TRACE_ENABLED = os.getenv("CAMPUSCLAW_TEST_TRACE", "false").lower() == "true"
TRACES: list[dict[str, Any]] = []
TRACE_LOCK = threading.Lock()
SCENARIO_PATTERN = re.compile(r"^[A-Za-z0-9._:-]{1,80}$")


def _error(request_id: Any, code: int, message: str) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": request_id, "error": {"code": code, "message": message}}


def _result(request_id: Any, result: dict[str, Any]) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": request_id, "result": result}


def _tool_call(name: str, arguments: Any) -> tuple[str | None, str | None]:
    if not isinstance(arguments, dict):
        return None, "arguments must be an object"
    if name == "formula_hint":
        if set(arguments) != {"topic"} or not isinstance(arguments["topic"], str):
            return None, "topic is required and must be text"
        topic = arguments["topic"].strip().lower()
        if not topic or len(topic) > 80:
            return None, "topic must contain 1 to 80 characters"
        hint = FORMULAS.get(topic)
        if not hint:
            return None, "unsupported topic; supported topics: speed, density, area, force"
        return f"解题线索：{hint}", None
    if name == "unit_convert":
        if set(arguments) != {"value", "from_unit", "to_unit"}:
            return None, "value, from_unit and to_unit are required"
        value, source, target = arguments["value"], arguments["from_unit"], arguments["to_unit"]
        if isinstance(value, bool) or not isinstance(value, (int, float)):
            return None, "value must be a finite number"
        if not (-1e12 < value < 1e12):
            return None, "value is outside the supported range"
        if source not in UNIT_GROUPS or target not in UNIT_GROUPS:
            return None, "unsupported unit"
        source_group, source_scale = UNIT_GROUPS[source]
        target_group, target_scale = UNIT_GROUPS[target]
        if source_group != target_group:
            return None, "units must belong to the same dimension"
        converted = value * source_scale / target_scale
        return f"{value:g} {source} = {converted:.10g} {target}", None
    return None, "unknown tool"


def dispatch(message: Any) -> dict[str, Any]:
    if not isinstance(message, dict) or message.get("jsonrpc") != "2.0" or not isinstance(message.get("method"), str):
        return _error(message.get("id") if isinstance(message, dict) else None, -32600, "Invalid Request")
    request_id = message.get("id")
    method = message["method"]
    params = message.get("params") or {}
    if method == "initialize":
        return _result(request_id, {
            "protocolVersion": PROTOCOL_VERSION,
            "capabilities": {"tools": {"listChanged": False}},
            "serverInfo": {"name": "campusclaw-mcp-demo", "version": "1.0.0"},
        })
    if method == "notifications/initialized":
        return _result(request_id, {}) if request_id is not None else {}
    if method == "tools/list":
        return _result(request_id, {"tools": TOOLS})
    if method == "tools/call":
        if not isinstance(params, dict) or not isinstance(params.get("name"), str):
            return _error(request_id, -32602, "Invalid params")
        text, error = _tool_call(params["name"], params.get("arguments"))
        if error:
            return _error(request_id, -32602, error)
        return _result(request_id, {"content": [{"type": "text", "text": text}], "isError": False})
    return _error(request_id, -32601, "Method not found")


class Handler(BaseHTTPRequestHandler):
    server_version = "CampusClawMCP/1.0"

    def log_message(self, _format: str, *_args: Any) -> None:
        # Never log headers or request bodies: they can contain credentials/student text.
        return

    def _json(self, status: int, value: Any) -> None:
        body = json.dumps(value, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        if self.path == "/__test/traces" and TRACE_ENABLED:
            with TRACE_LOCK:
                self._json(200, {"traces": list(TRACES)})
            return
        if self.path != "/health":
            self._json(404, {"error": "not found"})
            return
        configured = bool(os.environ.get("MCP_AUTH_TOKEN", ""))
        self._json(200 if configured else 503, {"status": "ok" if configured else "not_configured", "auth_configured": configured})

    def do_POST(self) -> None:
        if self.path != "/mcp":
            self._json(404, {"error": "not found"})
            return
        expected = os.environ.get("MCP_AUTH_TOKEN", "")
        if not expected:
            self._json(503, {"error": "MCP authentication is not configured"})
            return
        provided = self.headers.get("Authorization", "")
        if not provided.startswith("Bearer ") or not hmac.compare_digest(provided[7:].encode("utf-8"), expected.encode("utf-8")):
            self._json(401, {"error": "authentication required"})
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            if length <= 0 or length > MAX_BODY:
                raise ValueError("invalid size")
            message = json.loads(self.rfile.read(length))
        except (ValueError, json.JSONDecodeError):
            self._json(400, {"error": "invalid JSON-RPC request"})
            return
        response = dispatch(message)
        scenario = self.headers.get("X-CampusClaw-Test-Scenario", "")
        if TRACE_ENABLED and SCENARIO_PATTERN.fullmatch(scenario):
            item = {
                "trace_id": str(uuid.uuid4()),
                "scenario": scenario,
                "url": "http://mcp-demo:8090/mcp",
                "method": "POST",
                "timestamp": datetime.now(timezone.utc).isoformat(),
                "status": 200,
                "request": message,
                "response": response,
            }
            with TRACE_LOCK:
                TRACES.append(item)
                del TRACES[:-200]
        self._json(200, response)


def main() -> None:
    ThreadingHTTPServer(("0.0.0.0", 8090), Handler).serve_forever()


if __name__ == "__main__":
    main()
