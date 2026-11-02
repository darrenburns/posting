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
    restored = reopened.load(entry.id).response
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
    restored = store.load(store.entries()[0].id).response
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


def test_request_snapshot_round_trip_is_detached_and_immutable(tmp_path):
    from posting.collection import (
        Auth,
        Cookie,
        Header,
        Options,
        RequestBody,
        RequestModel,
        Scripts,
    )

    store = HistoryStore(tmp_path, tmp_path / "history")
    request = RequestModel(
        method="POST",
        url="${BASE_URL}/items",
        path=tmp_path / "saved.posting.yaml",
        body=RequestBody(content='{"token":"$TOKEN"}'),
        headers=[Header(name="X-Token", value="$TOKEN", enabled=False)],
        auth=Auth.basic_auth("user", "password"),
        options=Options(timeout=17),
        scripts=Scripts(on_request="deleted.py:function"),
        cookies=[Cookie(name="session", value="live")],
    )
    expected = request.model_dump()
    assert store.record(response(), request)
    request.headers[0].value = "changed"
    request.body.content = "changed"
    (entry,) = store.entries()
    assert entry.has_request
    restored = store.load(entry.id).request
    assert restored.model_dump() == expected
    assert restored.path is None
    assert restored.cookies == []


def test_response_only_database_migrates_without_losing_entries(tmp_path):
    from posting.collection import RequestModel

    store = HistoryStore(tmp_path, tmp_path / "history")
    store.record(response())
    # Recreate the schema shipped in the response-only version of this feature.
    with sqlite3.connect(store.path) as connection:
        connection.execute("ALTER TABLE responses DROP COLUMN request_json")
    reopened = HistoryStore(tmp_path, tmp_path / "history")
    (entry,) = reopened.entries()
    assert not entry.has_request
    assert reopened.load(entry.id).request is None
    assert reopened.load(entry.id).response.content == response().content
    assert reopened.record(response(), RequestModel(url="https://example.test/items"))
    assert [entry.has_request for entry in reopened.entries()] == [True, False]


def test_request_configuration_counts_toward_byte_budget(tmp_path):
    from posting.collection import RequestBody, RequestModel

    store = HistoryStore(tmp_path, tmp_path / "history", max_bytes=1000)
    assert not store.record(
        response(), RequestModel(body=RequestBody(content="x" * 1000))
    )
    assert store.entries() == []
