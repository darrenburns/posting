import pytest

from posting.graphql import cache
from posting.graphql.schema import (
    INTROSPECTION_QUERY,
    Schema,
    SchemaError,
    named_type_of,
    parse_introspection,
    render_type_ref,
)


class TestRenderTypeRef:
    @pytest.mark.parametrize(
        "type_ref,expected",
        [
            ({"kind": "SCALAR", "name": "String"}, "String"),
            ({"kind": "NON_NULL", "ofType": {"kind": "SCALAR", "name": "ID"}}, "ID!"),
            (
                {
                    "kind": "LIST",
                    "ofType": {
                        "kind": "NON_NULL",
                        "ofType": {"kind": "OBJECT", "name": "Post"},
                    },
                },
                "[Post!]",
            ),
            (
                {
                    "kind": "NON_NULL",
                    "ofType": {
                        "kind": "LIST",
                        "ofType": {
                            "kind": "NON_NULL",
                            "ofType": {"kind": "OBJECT", "name": "Post"},
                        },
                    },
                },
                "[Post!]!",
            ),
            (None, ""),
        ],
    )
    def test_rendering(self, type_ref, expected):
        assert render_type_ref(type_ref) == expected

    def test_deeply_wrapped_types(self):
        """A type reference can hold several list and non-null wrappers."""
        type_ref = {"kind": "OBJECT", "name": "Post"}
        for _ in range(4):
            type_ref = {
                "kind": "LIST",
                "ofType": {"kind": "NON_NULL", "ofType": type_ref},
            }

        assert render_type_ref(type_ref) == "[[[[Post!]!]!]!]"
        assert named_type_of(type_ref) == "Post"

    def test_the_introspection_query_asks_deep_enough_for_them(self):
        """`TypeRef` must be nested at least as deeply as we claim to support."""
        assert INTROSPECTION_QUERY.count("ofType") == 9

    def test_a_truncated_type_reference_is_not_mistaken_for_a_type(self):
        """A type nested deeper than the query asks for has no usable name."""
        truncated = {"kind": "LIST", "name": None}

        assert named_type_of(truncated) == ""

    def test_named_type_unwraps_wrappers(self):
        type_ref = {
            "kind": "NON_NULL",
            "ofType": {"kind": "LIST", "ofType": {"kind": "OBJECT", "name": "Post"}},
        }

        assert named_type_of(type_ref) == "Post"


class TestParseIntrospection:
    def test_root_types(self, schema: Schema):
        assert schema.query_type == "Query"
        assert schema.mutation_type == "Mutation"
        assert schema.subscription_type is None
        assert schema.root_type("query") is schema.type_named("Query")
        assert schema.root_type("mutation") is schema.type_named("Mutation")
        assert schema.root_type("subscription") is None

    def test_fields_and_arguments(self, schema: Schema):
        user_field = schema.field("Query", "user")

        assert user_field is not None
        assert user_field.type == "User"
        assert user_field.description == "Look a user up by ID."
        assert [argument.name for argument in user_field.arguments] == ["id", "role"]
        assert user_field.argument("id").type == "ID!"
        assert user_field.argument("nope") is None

    def test_list_field_types_are_rendered(self, schema: Schema):
        users = schema.field("Query", "users")

        assert users.type == "[User!]"
        assert users.named_type == "User"

    def test_deprecation_is_recorded(self, schema: Schema):
        assert schema.field("User", "email").deprecated is True
        assert schema.field("User", "name").deprecated is False

    def test_enum_values(self, schema: Schema):
        role = schema.type_named("Role")

        assert [value.name for value in role.enum_values] == ["ADMIN", "MEMBER"]
        assert role.enum_values[1].deprecated is True

    def test_input_object_fields(self, schema: Schema):
        user_input = schema.type_named("UserInput")

        assert list(user_input.input_fields) == ["name", "role", "profile"]
        assert user_input.input_fields["name"].type == "String!"

    def test_union_possible_types(self, schema: Schema):
        assert schema.type_named("SearchResult").possible_types == ("User", "Post")

    def test_introspection_types_are_excluded_from_counts(self, schema: Schema):
        assert "__Type" in schema.types
        assert schema.type_count == len(schema.types) - 1
        assert "__Type" not in schema.object_type_names()

    def test_errors_in_response_are_raised(self):
        with pytest.raises(SchemaError, match="Introspection is disabled"):
            parse_introspection({"errors": [{"message": "Introspection is disabled"}]})

    def test_response_without_schema_is_rejected(self):
        with pytest.raises(SchemaError, match="didn't contain a GraphQL schema"):
            parse_introspection({"data": {"user": None}})

    def test_schema_without_types_is_rejected(self):
        with pytest.raises(SchemaError, match="no types"):
            parse_introspection({"data": {"__schema": {"types": []}}})


class TestFindFields:
    def test_matching_field_names(self, schema: Schema):
        matches, total = schema.find_fields("title")

        assert [match.qualified_name for match in matches] == ["Post.title"]
        assert total == 1

    def test_matching_is_case_insensitive(self, schema: Schema):
        matches, _ = schema.find_fields("TITLE")

        assert [match.qualified_name for match in matches] == ["Post.title"]

    def test_partial_matches(self, schema: Schema):
        matches, _ = schema.find_fields("post")

        assert [match.qualified_name for match in matches] == [
            "User.posts",
            "Post.author",
            "Post.title",
        ]

    def test_names_starting_with_the_search_come_first(self, schema: Schema):
        matches, _ = schema.find_fields("na")

        assert [match.qualified_name for match in matches][0] == "User.name"

    def test_qualified_names_are_matched(self, schema: Schema):
        matches, _ = schema.find_fields("user.na")

        assert [match.qualified_name for match in matches] == ["User.name"]

    def test_types_can_be_excluded(self, schema: Schema):
        matches, total = schema.find_fields("user", exclude_types={"Query"})

        assert "Query.user" not in [match.qualified_name for match in matches]
        assert total == len(matches)

    def test_introspection_types_are_never_searched(self, schema: Schema):
        matches, _ = schema.find_fields("")

        assert matches == ()

    def test_limit_caps_the_results_but_not_the_total(self, schema: Schema):
        matches, total = schema.find_fields("post", limit=1)

        assert len(matches) == 1
        assert total == 3

    def test_no_matches(self, schema: Schema):
        assert schema.find_fields("nothingmatchesthis") == ((), 0)

    def test_the_matched_field_is_returned(self, schema: Schema):
        (match,), _ = schema.find_fields("email")

        assert match.type_name == "User"
        assert match.field.name == "email"
        assert match.field.deprecated is True


class TestCache:
    @pytest.fixture(autouse=True)
    def isolated_data_directory(self, monkeypatch, tmp_path):
        monkeypatch.setenv("XDG_DATA_HOME", str(tmp_path))
        monkeypatch.setattr(cache, "_schemas", {})
        monkeypatch.setattr(cache, "_disk_misses", set())

    def test_store_then_get(self, introspection_response):
        stored = cache.store_schema(
            "https://example.com/graphql", introspection_response
        )
        cached = cache.get_schema("https://example.com/graphql")

        assert cached is stored
        assert cached.schema.query_type == "Query"

    def test_unknown_url_returns_none(self):
        assert cache.get_schema("https://example.com/graphql") is None
        assert cache.get_schema("") is None

    def test_schema_is_loaded_from_disk(self, introspection_response, monkeypatch):
        cache.store_schema("https://example.com/graphql", introspection_response)

        # Simulate a restart by dropping the in-memory cache.
        monkeypatch.setattr(cache, "_schemas", {})
        monkeypatch.setattr(cache, "_disk_misses", set())

        cached = cache.get_schema("https://example.com/graphql")
        assert cached is not None
        assert cached.schema.field("Query", "user") is not None

    def test_schemas_are_cached_per_url(self, introspection_response):
        cache.store_schema("https://one.example.com/graphql", introspection_response)

        assert cache.get_schema("https://two.example.com/graphql") is None

    def test_invalid_cache_file_is_ignored(self, introspection_response, monkeypatch):
        url = "https://example.com/graphql"
        cache.store_schema(url, introspection_response)
        cache.schema_path(url).write_text("not json", encoding="utf-8")
        monkeypatch.setattr(cache, "_schemas", {})
        monkeypatch.setattr(cache, "_disk_misses", set())

        assert cache.get_schema(url) is None
