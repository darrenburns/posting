"""A small fake API for the sample-api demo collection.

Usage: python3 demo/server.py [PORT]   (default 8765)

Responses are canned, with a short delay so the trace shows some timing.
An X-Request-Id header on the request is echoed back on the response.
"""

from __future__ import annotations

import json
import sys
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

USERS = [
    {"id": 1, "name": "Ada Lovelace", "email": "ada@example.com", "role": "admin"},
    {"id": 2, "name": "Alan Turing", "email": "alan@example.com", "role": "editor"},
    {"id": 3, "name": "Katherine Johnson", "email": "katherine@example.com", "role": "viewer"},
]


class Handler(BaseHTTPRequestHandler):
    def version_string(self) -> str:
        return "sample-api/1.0"

    def log_message(self, format: str, *args: object) -> None:
        pass

    def respond(self, status: int, payload: object | None = None, extra: dict[str, str] | None = None) -> None:
        time.sleep(0.12)
        body = b"" if payload is None else json.dumps(payload).encode()
        self.send_response(status)
        if payload is not None:
            self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        if request_id := self.headers.get("X-Request-Id"):
            self.send_header("X-Request-Id", request_id)
        for name, value in (extra or {}).items():
            self.send_header(name, value)
        self.end_headers()
        self.wfile.write(body)

    def read_json(self) -> dict:
        length = int(self.headers.get("Content-Length") or 0)
        try:
            return json.loads(self.rfile.read(length) or b"{}")
        except json.JSONDecodeError:
            return {}

    def do_GET(self) -> None:
        path = self.path.split("?")[0].rstrip("/")
        if path == "/health":
            self.respond(200, {"status": "ok"})
        elif path == "/users":
            self.respond(200, {"users": USERS, "page": 1, "next": "/users?page=2&per_page=3"})
        elif path.startswith("/users/"):
            self.respond(200, USERS[0])
        elif path == "/auth/me":
            self.respond(200, USERS[0])
        elif path.startswith("/orders"):
            self.respond(200, {"id": 1042, "user_id": 1, "status": "shipped", "total": 31.98})
        else:
            self.respond(404, {"error": "not found"})

    def do_POST(self) -> None:
        path = self.path.split("?")[0].rstrip("/")
        if path == "/users":
            user = {"id": 4, **self.read_json(), "created_at": "2026-10-10T09:41:00Z"}
            self.respond(201, user, {"Location": "/users/4", "Set-Cookie": "session=8c1f; Path=/; HttpOnly"})
        elif path == "/auth/login":
            self.respond(200, {"token": "dev-token-123", "expires_in": 3600})
        elif path == "/orders":
            self.respond(201, {"id": 1043, **self.read_json(), "status": "pending"})
        else:
            self.respond(204)

    def do_PATCH(self) -> None:
        self.respond(200, {**USERS[0], **self.read_json()})

    def do_DELETE(self) -> None:
        self.respond(204)


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8765
    ThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()
