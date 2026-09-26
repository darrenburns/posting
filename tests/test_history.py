import gzip
import sqlite3
import stat
from datetime import timedelta

import httpx
import pytest

from posting.history import HistoryStore


def response(body=b'{"saved":true}', *, url="https://example.test/items", status=200):
    result = httpx.Response(
        status,
        content=body,
        headers=[
            ("content-type", "application/json"),
            ("set-cookie", "one=1"),
            ("set-cookie", "two=2"),
        ],
        request=httpx.Request(
            "POST", url, headers={"authorization": "secret"}, content=b"private"
        ),
    )
    result.elapsed = timedelta(milliseconds=42)
    return result


def test_round_trip_survives_restart_and_preserves_response(tmp_path):
    store = HistoryStore(tmp_path / "collection", tmp_path / "history")
    original = response()
    original.headers = httpx.Headers([*original.headers.raw, (b"X-Label", b"caf\xe9")])
    assert store.record(original)
    reopened = HistoryStore(tmp_path / "collection", tmp_path / "history")
    (entry,) = reopened.entries()
    restored = reopened.load(entry.id)
    assert restored.content == original.content
    assert restored.headers.raw == original.headers.raw
    assert dict(restored.cookies) == {"one": "1", "two": "2"}
    assert restored.elapsed == original.elapsed
    assert restored.status_code == original.status_code
    assert restored.request.method == "POST"
    assert restored.request.url == original.request.url
    assert "authorization" not in restored.request.headers
    assert restored.request.content == b""
    assert stat.S_IMODE(store.path.stat().st_mode) == 0o600
    assert stat.S_IMODE(store.path.parent.stat().st_mode) == 0o700


def test_compressed_binary_and_custom_metadata_round_trip(tmp_path):
    store = HistoryStore(tmp_path, tmp_path / "history")
    body = b"\xff\x00\x80binary"
    original = httpx.Response(
        201,
        content=gzip.compress(body),
        headers={
            "content-encoding": "gzip",
            "content-type": "application/octet-stream",
        },
        request=httpx.Request("GET", "https://example.test/download"),
        extensions={"http_version": b"HTTP/2", "reason_phrase": b"Custom reason"},
    )
    original.elapsed = timedelta(seconds=1)
    original.encoding = "latin-1"
    store.record(original)
    restored = store.load(store.entries()[0].id)
    assert restored.content == body
    assert restored.text == body.decode("latin-1")
    assert restored.http_version == "HTTP/2"
    assert restored.reason_phrase == "Custom reason"
    assert restored.headers["content-encoding"] == "gzip"


def test_collection_isolation_and_canonical_paths(tmp_path):
    first = HistoryStore(tmp_path / "one", tmp_path / "history")
    second = HistoryStore(tmp_path / "two", tmp_path / "history")
    first.record(response())
    assert second.entries() == []
    assert not second.path.exists()
    equivalent = HistoryStore(tmp_path / "one" / ".." / "one", tmp_path / "history")
    assert equivalent.entries() == first.entries()


def test_retention_delete_clear_and_multiple_connections(tmp_path):
    store = HistoryStore(tmp_path, tmp_path / "history", max_entries=2)
    other = HistoryStore(tmp_path, tmp_path / "history", max_entries=2)
    for status in (200, 404, 500):
        other.record(response(status=status))
    assert [entry.status_code for entry in store.entries()] == [500, 404]
    store.delete(store.entries()[0].id)
    assert [entry.status_code for entry in other.entries()] == [404]
    other.clear()
    assert store.entries() == []
    assert store.load(1) is None


def test_byte_budget_prunes_oldest_and_skips_oversize(tmp_path):
    store = HistoryStore(tmp_path, tmp_path / "history", max_bytes=1000)
    for status in (200, 201, 202):
        store.record(response(b"a" * 400, status=status))
    assert [entry.status_code for entry in store.entries()] == [202]
    assert not store.record(response(b"b" * 1001))
    assert [entry.status_code for entry in store.entries()] == [202]


def test_corrupt_database_is_not_overwritten(tmp_path):
    store = HistoryStore(tmp_path, tmp_path / "history")
    store.path.parent.mkdir()
    store.path.write_bytes(b"not a sqlite database")
    with pytest.raises(sqlite3.DatabaseError):
        store.record(response())
    assert store.path.read_bytes() == b"not a sqlite database"
