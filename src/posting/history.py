"""Bounded, collection-scoped response history, kept outside collection files."""

import hashlib
import json
import sqlite3
from contextlib import closing
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from pathlib import Path

import httpx

from posting.collection import RequestModel
from posting.locations import data_directory


@dataclass(frozen=True)
class HistoryEntry:
    id: int
    received_at: datetime
    method: str
    url: str
    status_code: int
    has_request: bool


@dataclass(frozen=True)
class HistoryRecord:
    response: httpx.Response
    request: RequestModel | None
    """None for entries saved before request snapshots were introduced."""


class HistoryStore:
    """Keep at most 100 responses / 50 MiB per collection in a private SQLite file.

    Connections are short-lived so separate Posting processes can share history.
    Bodies are stored as decoded bytes (never decoded a second time on replay).
    Request configuration is captured before variables and scripts are applied.
    """

    def __init__(
        self,
        collection: Path,
        directory: Path | None = None,
        *,
        max_entries: int = 100,
        max_bytes: int = 50 * 1024 * 1024,
    ) -> None:
        digest = hashlib.sha256(str(collection.resolve()).encode()).hexdigest()
        self.path = (directory or data_directory() / "history") / f"{digest}.sqlite3"
        self.max_entries = max_entries
        self.max_bytes = max_bytes

    def _connect(self) -> sqlite3.Connection:
        self.path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.path.touch(mode=0o600, exist_ok=True)
        connection = sqlite3.connect(self.path, timeout=0.25)
        connection.row_factory = sqlite3.Row
        try:
            # Reuse freed pages and erase deleted payloads, including on clear.
            connection.execute("PRAGMA secure_delete = ON")
            connection.execute("PRAGMA auto_vacuum = FULL")
            connection.execute(
                """CREATE TABLE IF NOT EXISTS responses (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    received_at TEXT NOT NULL,
                    method TEXT NOT NULL,
                    url TEXT NOT NULL,
                    status_code INTEGER NOT NULL,
                    headers TEXT NOT NULL,
                    body BLOB NOT NULL,
                    elapsed REAL NOT NULL,
                    encoding TEXT,
                    http_version TEXT NOT NULL,
                    reason_phrase TEXT NOT NULL,
                    size INTEGER NOT NULL,
                    request_json TEXT
                )"""
            )
            if "request_json" not in {
                row["name"]
                for row in connection.execute("PRAGMA table_info(responses)")
            }:
                # Recheck under a write lock in case another instance also migrates.
                with connection:
                    connection.execute("BEGIN IMMEDIATE")
                    columns = {
                        row["name"]
                        for row in connection.execute("PRAGMA table_info(responses)")
                    }
                    if "request_json" not in columns:
                        connection.execute(
                            "ALTER TABLE responses ADD COLUMN request_json TEXT"
                        )
        except Exception:
            connection.close()
            raise
        return connection

    def record(
        self, response: httpx.Response, request: RequestModel | None = None
    ) -> bool:
        """Save an exchange. Return False if it exceeds the byte budget."""
        headers = json.dumps(
            [
                (name.decode("latin-1"), value.decode("latin-1"))
                for name, value in response.headers.raw
            ],
            ensure_ascii=True,
        )
        url = str(response.request.url)
        request_json = request.model_dump_json() if request is not None else None
        size = (
            len(response.content)
            + len(headers)
            + len(url.encode())
            + (len(request_json.encode()) if request_json else 0)
        )
        if size > self.max_bytes:
            return False
        with closing(self._connect()) as connection, connection:
            connection.execute(
                """INSERT INTO responses
                (received_at, method, url, status_code, headers, body, elapsed,
                 encoding, http_version, reason_phrase, size, request_json)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
                (
                    datetime.now(timezone.utc).isoformat(),
                    response.request.method,
                    url,
                    response.status_code,
                    headers,
                    response.content,
                    response.elapsed.total_seconds(),
                    response.encoding,
                    response.http_version,
                    response.reason_phrase,
                    size,
                    request_json,
                ),
            )
            # Keep the newest contiguous set within both retention limits.
            rows = connection.execute(
                "SELECT id, size FROM responses ORDER BY id DESC"
            ).fetchall()
            total = 0
            for index, row in enumerate(rows):
                total += row["size"]
                if index >= self.max_entries or total > self.max_bytes:
                    connection.execute(
                        "DELETE FROM responses WHERE id <= ?", (row["id"],)
                    )
                    break
        return True

    def entries(self) -> list[HistoryEntry]:
        """List metadata only; response bodies are loaded on selection."""
        if not self.path.exists():
            return []
        with closing(self._connect()) as connection:
            return [
                HistoryEntry(
                    row["id"],
                    datetime.fromisoformat(row["received_at"]),
                    row["method"],
                    row["url"],
                    row["status_code"],
                    bool(row["has_request"]),
                )
                for row in connection.execute(
                    "SELECT id, received_at, method, url, status_code, request_json IS NOT NULL AS has_request FROM responses ORDER BY id DESC"
                )
            ]

    def load(self, entry_id: int) -> HistoryRecord | None:
        if not self.path.exists():
            return None
        with closing(self._connect()) as connection:
            row = connection.execute(
                "SELECT * FROM responses WHERE id = ?", (entry_id,)
            ).fetchone()
        if row is None:
            return None
        response = httpx.Response(
            row["status_code"],
            content=row["body"],
            request=httpx.Request(row["method"], row["url"]),
            extensions={
                "http_version": row["http_version"].encode("ascii"),
                "reason_phrase": row["reason_phrase"].encode("ascii"),
            },
        )
        # httpx normally decompresses content on construction. These bytes were
        # already decompressed when received, but retain the original headers.
        response.headers = httpx.Headers(json.loads(row["headers"]), encoding="latin-1")
        response.encoding = row["encoding"]
        response.elapsed = timedelta(seconds=row["elapsed"])
        request = (
            RequestModel.model_validate_json(row["request_json"])
            if row["request_json"] is not None
            else None
        )
        return HistoryRecord(response, request)

    def delete(self, entry_id: int) -> None:
        if self.path.exists():
            with closing(self._connect()) as connection, connection:
                connection.execute("DELETE FROM responses WHERE id = ?", (entry_id,))

    def clear(self) -> None:
        if self.path.exists():
            with closing(self._connect()) as connection, connection:
                connection.execute("DELETE FROM responses")
