import hashlib
import json
import math
import os
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

DIMENSION = int(os.getenv("EMBEDDING_DIMENSION", "16"))
COUNTERS = {"embeddings": 0, "chat": 0}
LAST = {"last_chat_messages": []}


def vector_for(text):
    lowered = text.lower()
    values = [0.0] * DIMENSION
    has_concept = False
    concepts = [
        (0, ("实验", "experiment", "做实验")),
        (1, ("安全", "培训", "准备", "之前", "前", "training", "prepare")),
        (2, ("太阳能", "钙钛矿", "电池")),
        (3, ("量子", "通信", "纠缠", "光子")),
        (4, ("课程", "学生", "讲义")),
    ]
    for index, words in concepts:
        if index < DIMENSION and any(word in lowered for word in words):
            values[index] += 4.0
            has_concept = True
    if not has_concept:
        return values
    digest = hashlib.sha256(text.encode("utf-8")).digest()
    for index in range(5, DIMENSION):
        values[index] = ((digest[index % len(digest)] / 255.0) - 0.5) * 0.08
    norm = math.sqrt(sum(value * value for value in values))
    return [value / norm for value in values]


class Handler(BaseHTTPRequestHandler):
    def log_message(self, _format, *_args):
        return

    def send_json(self, status, value):
        body = json.dumps(value, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def read_json(self):
        length = int(self.headers.get("Content-Length", "0"))
        if length > 1024 * 1024:
            raise ValueError("request too large")
        return json.loads(self.rfile.read(length) or b"{}")

    def do_GET(self):
        if self.path == "/health":
            self.send_json(200, {"status": "ok", "provider": "deterministic-mock"})
        elif self.path == "/stats":
            self.send_json(200, {**COUNTERS, **LAST})
        else:
            self.send_json(404, {"error": "not found"})

    def do_POST(self):
        try:
            request = self.read_json()
        except (ValueError, json.JSONDecodeError):
            self.send_json(400, {"error": "invalid request"})
            return
        if self.path == "/reset":
            COUNTERS.update(embeddings=0, chat=0)
            LAST["last_chat_messages"] = []
            self.send_json(200, {**COUNTERS, **LAST})
            return
        if self.path == "/embeddings":
            COUNTERS["embeddings"] += 1
            inputs = request.get("input", [])
            if isinstance(inputs, str):
                inputs = [inputs]
            data = [{"object": "embedding", "index": index, "embedding": vector_for(str(text))} for index, text in enumerate(inputs)]
            self.send_json(200, {"object": "list", "model": request.get("model", "mock"), "data": data})
            return
        if self.path == "/chat/completions":
            COUNTERS["chat"] += 1
            messages = request.get("messages", [])
            LAST["last_chat_messages"] = messages
            prompt = "\n".join(str(item.get("content", "")) for item in messages if item.get("role") == "user")
            markers = []
            for marker in ("[1]", "[2]", "[3]", "[4]"):
                if marker in prompt:
                    markers.append(marker)
            suffix = " ".join(markers[:2] or ["[1]"])
            content = f"根据本班资料，实验前需要先完成安全培训。{suffix}"
            if request.get("stream") is True:
                system_prompt = "\n".join(str(item.get("content", "")) for item in messages if item.get("role") == "system")
                if "禁止直接给出最终答案" in system_prompt:
                    content = f"我们先从资料线索开始：你能指出实验开始前必须完成的准备事项吗？{suffix}"
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream; charset=utf-8")
                self.send_header("Cache-Control", "no-cache")
                self.send_header("Connection", "keep-alive")
                self.end_headers()
                for token in (content[index:index + 2] for index in range(0, len(content), 2)):
                    event = {"choices": [{"index": 0, "delta": {"content": token}, "finish_reason": None}]}
                    self.wfile.write(("data: " + json.dumps(event, ensure_ascii=False) + "\n\n").encode("utf-8"))
                    self.wfile.flush()
                    time.sleep(0.01)
                self.wfile.write(b"data: [DONE]\n\n")
                self.wfile.flush()
                return
            self.send_json(200, {"id": "mock-chat", "object": "chat.completion", "choices": [{"index": 0, "message": {"role": "assistant", "content": content}, "finish_reason": "stop"}]})
            return
        self.send_json(404, {"error": "not found"})


ThreadingHTTPServer(("0.0.0.0", 8081), Handler).serve_forever()
