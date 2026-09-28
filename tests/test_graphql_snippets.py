import pytest

from posting.graphql.schema import Schema, parse_introspection
from posting.graphql.snippets import (
    MAX_FIELDS,
    SchemaSelection,
    build_operation,
    build_selection,
    operation_name_for,
    snippet_for,
)


class TestBuildOperation:
    def test_query_with_a_required_argument(self, schema: Schema):
        snippet = build_operation(schema, "query", "user")

        assert snippet.text == (
            "query GetUser($id: ID!) {\n"
            "  user(id: $id) {\n"
            "    id\n"
            "    name\n"
            "    posts {\n"
            "      title\n"
            "    }\n"
            "  }\n"
            "}"
        )
        assert snippet.variables == {"id": "ID!"}
        assert snippet.operation_name == "GetUser"

    def test_query_without_arguments(self, schema: Schema):
        snippet = build_operation(schema, "query", "users")

        assert snippet.text == (
            "query GetUsers {\n"
            "  users {\n"
            "    id\n"
            "    name\n"
            "    posts {\n"
            "      title\n"
            "    }\n"
            "  }\n"
            "}"
        )
        assert snippet.variables == {}

    def test_mutation(self, schema: Schema):
        snippet = build_operation(schema, "mutation", "createUser")

        assert snippet.operation_name == "CreateUser"
        assert snippet.text.startswith("mutation CreateUser($input: UserInput!) {")
        assert snippet.variables == {"input": "UserInput!"}

    def test_union_selects_typename(self, schema: Schema):
        snippet = build_operation(schema, "query", "search")

        assert snippet.text == (
            "query GetSearch($text: String!) {\n"
            "  search(text: $text) {\n"
            "    __typename\n"
            "  }\n"
            "}"
        )

    def test_deprecated_fields_are_left_out(self, schema: Schema):
        snippet = build_operation(schema, "query", "user")

        assert "email" not in snippet.text

    def test_optional_arguments_are_left_out(self, schema: Schema):
        # `user` also takes an optional `role` argument.
        snippet = build_operation(schema, "query", "user")

        assert "role" not in snippet.text

    def test_unknown_field(self, schema: Schema):
        assert build_operation(schema, "query", "nope") is None

    def test_operation_without_a_root_type(self, schema: Schema):
        assert build_operation(schema, "subscription", "user") is None

    def test_variables_json_template(self, schema: Schema):
        snippet = build_operation(schema, "query", "user")

        assert snippet.variables_json() == '{\n  "id": null\n}'

    def test_variables_json_is_empty_without_variables(self, schema: Schema):
        assert build_operation(schema, "query", "users").variables_json() == ""


class TestBuildSelection:
    def test_object_field(self, schema: Schema):
        snippet = build_selection(schema, "User", "posts")

        assert snippet.text == (
            "posts {\n  title\n  author {\n    id\n    name\n  }\n}"
        )

    def test_scalar_field_has_no_selection_set(self, schema: Schema):
        assert build_selection(schema, "User", "name").text == "name"

    def test_required_arguments_become_variables(self, schema: Schema):
        snippet = build_selection(schema, "Query", "search")

        assert snippet.text.startswith("search(text: $text) {")
        assert snippet.variables == {"text": "String!"}

    def test_nested_objects_are_expanded(self, schema: Schema):
        assert build_selection(schema, "Post", "author").text == (
            "author {\n  id\n  name\n  posts {\n    title\n  }\n}"
        )

    def test_recursion_stops_when_a_type_contains_itself(self, schema: Schema):
        """`User.posts.author` is a `User` again, so it isn't expanded further."""
        snippet = build_selection(schema, "User", "posts")

        assert snippet.text.count("author") == 1
        assert snippet.text.count("posts") == 1

    def test_unknown_field(self, schema: Schema):
        assert build_selection(schema, "User", "nope") is None
        assert build_selection(schema, "Nope", "name") is None


class TestOperationNames:
    def test_names(self):
        assert operation_name_for("query", "user") == "GetUser"
        assert operation_name_for("mutation", "createUser") == "CreateUser"
        assert operation_name_for("subscription", "userChanged") == "OnUserChanged"


class TestSnippetFor:
    def test_root_field_in_an_empty_document_becomes_an_operation(self, schema: Schema):
        selection = SchemaSelection("Query", "user", "query")

        snippet = snippet_for(schema, selection, "", 0)

        assert snippet.operation_name == "GetUser"
        assert snippet.text.startswith("query GetUser")

    def test_root_field_inside_a_selection_set_becomes_a_selection(
        self, schema: Schema
    ):
        document = "query {\n  user(id: $id) {\n    \n  }\n}"
        cursor = document.index("\n  }")
        selection = SchemaSelection("Query", "users", "query")

        snippet = snippet_for(schema, selection, document, cursor)

        assert snippet.operation_name == ""
        assert snippet.text.startswith("users {")

    def test_root_field_at_the_top_level_of_a_document_becomes_an_operation(
        self, schema: Schema
    ):
        document = "query GetUsers {\n  users {\n    id\n  }\n}\n"
        selection = SchemaSelection("Query", "user", "query")

        snippet = snippet_for(schema, selection, document, len(document))

        assert snippet.operation_name == "GetUser"

    def test_nested_field_is_always_a_selection(self, schema: Schema):
        selection = SchemaSelection("User", "posts", None)

        snippet = snippet_for(schema, selection, "", 0)

        assert snippet.operation_name == ""
        assert snippet.text.startswith("posts {")

    def test_unknown_field(self, schema: Schema):
        assert snippet_for(schema, SchemaSelection("User", "nope"), "", 0) is None


def _scalar(name: str) -> dict:
    return {"kind": "SCALAR", "name": name}


def _object(name: str) -> dict:
    return {"kind": "OBJECT", "name": name}


def _non_null(of_type: dict) -> dict:
    return {"kind": "NON_NULL", "ofType": of_type}


def _list(of_type: dict) -> dict:
    return {"kind": "LIST", "ofType": of_type}


@pytest.fixture
def nested_schema() -> Schema:
    """A schema whose types have object fields rather than scalar ones.

    `Dashboard` has no fields which can be selected on their own, and one of
    its object fields is a list which takes a required argument - the shape
    that pagination usually has.
    """
    return parse_introspection(
        {
            "data": {
                "__schema": {
                    "queryType": {"name": "Query"},
                    "types": [
                        {
                            "kind": "OBJECT",
                            "name": "Query",
                            "fields": [
                                {
                                    "name": "dashboard",
                                    "type": _object("Dashboard"),
                                    "args": [],
                                }
                            ],
                        },
                        {
                            "kind": "OBJECT",
                            "name": "Dashboard",
                            "fields": [
                                {
                                    "name": "owner",
                                    "type": _object("User"),
                                    "args": [],
                                },
                                {
                                    "name": "widgets",
                                    "type": _list(_object("Widget")),
                                    "args": [],
                                },
                                {
                                    "name": "pages",
                                    "type": _list(_non_null(_object("Page"))),
                                    "args": [
                                        {
                                            "name": "first",
                                            "type": _non_null(_scalar("Int")),
                                        }
                                    ],
                                },
                            ],
                        },
                        {
                            "kind": "OBJECT",
                            "name": "User",
                            "fields": [
                                {"name": "name", "type": _scalar("String")},
                                {
                                    "name": "avatar",
                                    "type": _scalar("String"),
                                    "args": [
                                        {
                                            "name": "size",
                                            "type": _non_null(_scalar("Int")),
                                        }
                                    ],
                                },
                            ],
                        },
                        {
                            "kind": "OBJECT",
                            "name": "Widget",
                            "fields": [{"name": "title", "type": _scalar("String")}],
                        },
                        {
                            "kind": "OBJECT",
                            "name": "Page",
                            "fields": [{"name": "slug", "type": _scalar("String")}],
                        },
                        _scalar("String"),
                        _scalar("Int"),
                    ],
                }
            }
        }
    )


class TestFieldsWithRequiredArguments:
    """Fields which take required arguments are still worth selecting."""

    def test_a_list_field_with_a_required_argument_is_included(
        self, nested_schema: Schema
    ):
        snippet = build_selection(nested_schema, "Query", "dashboard")

        assert snippet.text == (
            "dashboard {\n"
            "  owner {\n"
            "    name\n"
            "    avatar(size: $size)\n"
            "  }\n"
            "  widgets {\n"
            "    title\n"
            "  }\n"
            "  pages(first: $first) {\n"
            "    slug\n"
            "  }\n"
            "}"
        )

    def test_the_arguments_become_variables(self, nested_schema: Schema):
        snippet = build_operation(nested_schema, "query", "dashboard")

        assert snippet.variables == {"size": "Int!", "first": "Int!"}
        assert snippet.text.startswith(
            "query GetDashboard($size: Int!, $first: Int!) {"
        )

    def test_variable_names_do_not_collide(self, nested_schema: Schema):
        """Two fields taking the same argument name get separate variables."""
        snippet = build_selection(nested_schema, "Dashboard", "pages")
        second = build_operation(nested_schema, "query", "dashboard")

        assert snippet.variables == {"first": "Int!"}
        assert len(second.variables) == len(set(second.variables))


@pytest.fixture
def deep_schema() -> Schema:
    """A schema shaped like `getStuff { object1 { fields..., list { items } } }`.

    `Object1` has both fields which can be selected on their own *and* an
    object field, which is what a real API usually looks like.
    """
    return parse_introspection(
        {
            "data": {
                "__schema": {
                    "queryType": {"name": "Query"},
                    "types": [
                        {
                            "kind": "OBJECT",
                            "name": "Query",
                            "fields": [
                                {
                                    "name": "getStuff",
                                    "type": _object("Stuff"),
                                    "args": [],
                                }
                            ],
                        },
                        {
                            "kind": "OBJECT",
                            "name": "Stuff",
                            "fields": [
                                {
                                    "name": "object1",
                                    "type": _object("Object1"),
                                    "args": [],
                                }
                            ],
                        },
                        {
                            "kind": "OBJECT",
                            "name": "Object1",
                            "fields": [
                                {"name": "field1", "type": _scalar("String")},
                                {"name": "field2", "type": _scalar("String")},
                                {
                                    "name": "listOfObjects",
                                    "type": _object("ListOfObjects"),
                                    "args": [],
                                },
                            ],
                        },
                        {
                            "kind": "OBJECT",
                            "name": "ListOfObjects",
                            "fields": [
                                {
                                    "name": "items",
                                    "type": _list(_object("Item")),
                                    "args": [],
                                }
                            ],
                        },
                        {
                            "kind": "OBJECT",
                            "name": "Item",
                            "fields": [
                                {"name": "itemField1", "type": _scalar("String")},
                                {"name": "itemField2", "type": _scalar("String")},
                            ],
                        },
                        _scalar("String"),
                    ],
                }
            }
        }
    )


class TestNestedObjectsAlongsideFields:
    """Objects are selected alongside the fields which need no selection set.

    Selecting only the latter would quietly drop whole branches of the graph.
    """

    def test_a_root_field_expands_the_whole_shape(self, deep_schema: Schema):
        assert build_selection(deep_schema, "Query", "getStuff").text == (
            "getStuff {\n"
            "  object1 {\n"
            "    field1\n"
            "    field2\n"
            "    listOfObjects {\n"
            "      items {\n"
            "        itemField1\n"
            "        itemField2\n"
            "      }\n"
            "    }\n"
            "  }\n"
            "}"
        )

    def test_an_intermediate_object_expands_the_whole_shape(self, deep_schema: Schema):
        assert build_selection(deep_schema, "Stuff", "object1").text == (
            "object1 {\n"
            "  field1\n"
            "  field2\n"
            "  listOfObjects {\n"
            "    items {\n"
            "      itemField1\n"
            "      itemField2\n"
            "    }\n"
            "  }\n"
            "}"
        )

    def test_the_operation_includes_the_nested_list(self, deep_schema: Schema):
        snippet = build_operation(deep_schema, "query", "getStuff")

        assert "listOfObjects {" in snippet.text
        assert "items {" in snippet.text
        assert snippet.truncated is False

    def test_objects_deeper_than_the_depth_limit_are_left_out(
        self, deep_schema: Schema
    ):
        snippet = build_selection(deep_schema, "Query", "getStuff", depth=2)

        assert "field1" in snippet.text
        assert "listOfObjects" not in snippet.text

    def test_a_selection_set_which_would_be_empty_selects_typename(
        self, deep_schema: Schema
    ):
        # At depth 1 there's no room to expand `object1`, and `Stuff` has
        # nothing else to select.
        assert build_selection(deep_schema, "Query", "getStuff", depth=1).text == (
            "getStuff {\n  __typename\n}"
        )


class TestVeryWideTypes:
    @pytest.fixture
    def wide_schema(self) -> Schema:
        """A type with far more fields than one selection should contain."""
        return parse_introspection(
            {
                "data": {
                    "__schema": {
                        "queryType": {"name": "Query"},
                        "types": [
                            {
                                "kind": "OBJECT",
                                "name": "Query",
                                "fields": [
                                    {
                                        "name": "wide",
                                        "type": _object("Wide"),
                                        "args": [],
                                    }
                                ],
                            },
                            {
                                "kind": "OBJECT",
                                "name": "Wide",
                                "fields": [
                                    {"name": f"field{index}", "type": _scalar("String")}
                                    for index in range(500)
                                ],
                            },
                            _scalar("String"),
                        ],
                    }
                }
            }
        )

    def test_the_selection_is_capped(self, wide_schema: Schema):
        snippet = build_selection(wide_schema, "Query", "wide")

        assert snippet.truncated is True
        assert snippet.text.count("field") <= MAX_FIELDS

    def test_what_is_generated_is_still_valid(self, wide_schema: Schema):
        snippet = build_selection(wide_schema, "Query", "wide")

        assert snippet.text.startswith("wide {")
        assert snippet.text.endswith("}")
