from pathlib import Path

import pytest

from posting.graphql import cache
from posting.graphql.schema import Schema, parse_introspection


def scalar(name: str) -> dict:
    return {"kind": "SCALAR", "name": name}


def obj(name: str) -> dict:
    return {"kind": "OBJECT", "name": name}


def non_null(of_type: dict) -> dict:
    return {"kind": "NON_NULL", "ofType": of_type}


def list_of(of_type: dict) -> dict:
    return {"kind": "LIST", "ofType": of_type}


INTROSPECTION_RESPONSE = {
    "data": {
        "__schema": {
            "queryType": {"name": "Query"},
            "mutationType": {"name": "Mutation"},
            "subscriptionType": None,
            "types": [
                {
                    "kind": "OBJECT",
                    "name": "Query",
                    "description": "The query root.",
                    "fields": [
                        {
                            "name": "user",
                            "description": "Look a user up by ID.",
                            "type": obj("User"),
                            "args": [
                                {"name": "id", "type": non_null(scalar("ID"))},
                                {
                                    "name": "role",
                                    "type": {"kind": "ENUM", "name": "Role"},
                                },
                            ],
                        },
                        {
                            "name": "users",
                            "type": list_of(non_null(obj("User"))),
                            "args": [],
                        },
                        {
                            "name": "search",
                            "type": {"kind": "UNION", "name": "SearchResult"},
                            "args": [
                                {"name": "text", "type": non_null(scalar("String"))}
                            ],
                        },
                    ],
                },
                {
                    "kind": "OBJECT",
                    "name": "Mutation",
                    "fields": [
                        {
                            "name": "createUser",
                            "type": obj("User"),
                            "args": [
                                {
                                    "name": "input",
                                    "type": non_null(
                                        {"kind": "INPUT_OBJECT", "name": "UserInput"}
                                    ),
                                }
                            ],
                        }
                    ],
                },
                {
                    "kind": "OBJECT",
                    "name": "User",
                    "fields": [
                        {"name": "id", "type": non_null(scalar("ID"))},
                        {"name": "name", "type": scalar("String")},
                        {
                            "name": "email",
                            "type": scalar("String"),
                            "isDeprecated": True,
                        },
                        {
                            "name": "posts",
                            "type": list_of(obj("Post")),
                            "args": [{"name": "first", "type": scalar("Int")}],
                        },
                    ],
                },
                {
                    "kind": "OBJECT",
                    "name": "Post",
                    "fields": [
                        {"name": "title", "type": scalar("String")},
                        {"name": "author", "type": obj("User")},
                    ],
                },
                {
                    "kind": "ENUM",
                    "name": "Role",
                    "enumValues": [
                        {"name": "ADMIN"},
                        {"name": "MEMBER", "isDeprecated": True},
                    ],
                },
                {
                    "kind": "INPUT_OBJECT",
                    "name": "UserInput",
                    "inputFields": [
                        {"name": "name", "type": non_null(scalar("String"))},
                        {"name": "role", "type": {"kind": "ENUM", "name": "Role"}},
                        {
                            "name": "profile",
                            "type": {"kind": "INPUT_OBJECT", "name": "ProfileInput"},
                        },
                    ],
                },
                {
                    "kind": "INPUT_OBJECT",
                    "name": "ProfileInput",
                    "inputFields": [{"name": "city", "type": scalar("String")}],
                },
                {
                    "kind": "UNION",
                    "name": "SearchResult",
                    "possibleTypes": [obj("User"), obj("Post")],
                },
                {"kind": "SCALAR", "name": "ID"},
                {"kind": "SCALAR", "name": "String"},
                {"kind": "SCALAR", "name": "Int"},
                {"kind": "OBJECT", "name": "__Type", "fields": []},
            ],
        }
    }
}
"""An introspection response for a small schema, used across the GraphQL tests."""


@pytest.fixture
def introspection_response() -> dict:
    return INTROSPECTION_RESPONSE


@pytest.fixture
def schema() -> Schema:
    return parse_introspection(INTROSPECTION_RESPONSE)


@pytest.fixture(autouse=True)
def clear_graphql_schema_cache():
    """Keep fetched schemas from leaking between tests.

    The cache lives for the lifetime of the process, so without this a test
    which stores a schema changes what later tests see.
    """
    cache._schemas.clear()
    cache._disk_misses.clear()
    yield
    cache._schemas.clear()
    cache._disk_misses.clear()


TESTS_DIR = Path(__file__).parent


@pytest.fixture
def anyio_backend() -> str:
    """The backend `@pytest.mark.anyio` tests run on."""
    return "asyncio"


@pytest.fixture
def app_environment(monkeypatch, tmp_path):
    """Keep an app under test out of the user's config and data directories."""
    monkeypatch.setenv(
        "POSTING_CONFIG_FILE", str(TESTS_DIR / "sample-configs" / "general.yaml")
    )
    monkeypatch.setenv("XDG_DATA_HOME", str(tmp_path / "data"))


@pytest.fixture
def open_body_tab():
    """Returns a function which shows the request body tab of a running app.

    The tab is loaded lazily, so its widgets aren't available until it has
    been shown.
    """
    from posting.widgets.request.request_editor import RequestEditorTabbedContent

    async def open_tab(pilot):
        screen = pilot.app.screen
        tabs = screen.request_editor.query_one(RequestEditorTabbedContent)
        tabs.active = "body-pane"
        for _ in range(5):
            await pilot.pause()
        return screen

    return open_tab
