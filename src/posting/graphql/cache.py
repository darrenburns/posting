"""Caching of fetched GraphQL schemas.

Schemas are cached in memory for the lifetime of the app, and on disk inside
Posting's data directory so that autocompletion keeps working across restarts
without needing to re-introspect the endpoint.
"""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from textual import log

from posting.graphql.schema import Schema, parse_introspection
from posting.locations import data_directory


@dataclass(frozen=True)
class CachedSchema:
    """A schema which was fetched from an endpoint."""

    url: str
    schema: Schema
    fetched_at: datetime


_schemas: dict[str, CachedSchema] = {}
"""In-memory cache of schemas, keyed by endpoint URL."""

_disk_misses: set[str] = set()
"""URLs which we've already looked for on disk and failed to find."""


def schema_directory() -> Path:
    """The directory which holds cached schemas.

    The directory isn't created here - looking a schema up shouldn't have any
    side effects on disk. It's created when a schema is written.
    """
    return data_directory() / "graphql-schemas"


def schema_path(url: str) -> Path:
    """The path of the file which caches the schema for the given URL."""
    digest = hashlib.sha256(url.encode("utf-8")).hexdigest()[:16]
    return schema_directory() / f"{digest}.json"


def store_schema(url: str, introspection: dict[str, Any]) -> CachedSchema:
    """Parse an introspection response, and cache the resulting schema.

    Args:
        url: The endpoint the schema was fetched from.
        introspection: The decoded JSON body of the introspection response.

    Returns:
        The cached schema.

    Raises:
        SchemaError: If the response doesn't contain a schema.
    """
    cached = CachedSchema(
        url=url,
        schema=parse_introspection(introspection),
        fetched_at=datetime.now(timezone.utc),
    )
    _schemas[url] = cached
    _disk_misses.discard(url)

    path = schema_path(url)
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(
            json.dumps(
                {
                    "url": url,
                    "fetched_at": cached.fetched_at.isoformat(),
                    "introspection": introspection,
                }
            ),
            encoding="utf-8",
        )
    except OSError as error:
        # A schema we can't write to disk is still perfectly usable in memory.
        log.warning(f"Could not cache GraphQL schema for {url!r}: {error}")

    return cached


def get_schema(url: str) -> CachedSchema | None:
    """Return the cached schema for a URL, if we have one.

    The in-memory cache is checked first. On the first miss for a URL, the
    on-disk cache is checked (and loaded into memory if it's present).
    """
    if not url:
        return None
    if cached := _schemas.get(url):
        return cached
    if url in _disk_misses:
        return None

    cached = _load_from_disk(url)
    if cached is None:
        _disk_misses.add(url)
    else:
        _schemas[url] = cached
    return cached


def _load_from_disk(url: str) -> CachedSchema | None:
    path = schema_path(url)
    try:
        raw = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None

    try:
        schema = parse_introspection(raw["introspection"])
        fetched_at = datetime.fromisoformat(raw["fetched_at"])
    except Exception as error:
        log.warning(f"Ignoring invalid cached GraphQL schema at {path}: {error}")
        return None

    return CachedSchema(url=url, schema=schema, fetched_at=fetched_at)
