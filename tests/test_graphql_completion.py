import pytest

from posting.graphql.completion import analyze, complete
from posting.graphql.schema import Schema


def names(schema: Schema, text: str, cursor: int | None = None) -> list[str]:
    """The names of the completions offered at the end of `text`."""
    return [
        completion.name
        for completion in complete(
            schema, text, len(text) if cursor is None else cursor
        )
    ]


class TestFieldCompletions:
    def test_root_fields_of_an_anonymous_query(self, schema: Schema):
        assert names(schema, "{ us") == ["user", "users", "search", "__typename"]

    def test_root_fields_of_a_named_query(self, schema: Schema):
        assert names(schema, "query GetUser { ") == [
            "user",
            "users",
            "search",
            "__typename",
        ]

    def test_mutation_root_fields(self, schema: Schema):
        assert names(schema, "mutation { ") == ["createUser", "__typename"]

    def test_nested_fields(self, schema: Schema):
        assert names(schema, "{ user { ") == [
            "id",
            "name",
            "posts",
            "email",
            "__typename",
        ]

    def test_deprecated_fields_are_offered_last(self, schema: Schema):
        completions = complete(schema, "{ user { ", len("{ user { "))
        deprecated = [c.name for c in completions if c.deprecated]

        assert deprecated == ["email"]
        assert completions[-2].name == "email"

    def test_fields_through_a_list_type(self, schema: Schema):
        assert names(schema, "{ user { posts { ") == ["title", "author", "__typename"]

    def test_fields_through_an_alias(self, schema: Schema):
        assert names(schema, "{ user { articles: posts { ") == [
            "title",
            "author",
            "__typename",
        ]

    def test_fields_after_a_field_with_arguments(self, schema: Schema):
        assert names(schema, "{ user(id: 1) { ") == [
            "id",
            "name",
            "posts",
            "email",
            "__typename",
        ]

    def test_fields_after_a_completed_sibling_selection(self, schema: Schema):
        assert names(schema, "{ users { name } user { ") == [
            "id",
            "name",
            "posts",
            "email",
            "__typename",
        ]

    def test_second_operation_in_a_document(self, schema: Schema):
        assert names(schema, "query { users { name } } mutation { ") == [
            "createUser",
            "__typename",
        ]

    def test_fragment_definition_uses_its_type_condition(self, schema: Schema):
        assert names(schema, "fragment Fields on Post { ") == [
            "title",
            "author",
            "__typename",
        ]

    def test_operation_named_after_a_type_still_uses_the_root(self, schema: Schema):
        assert names(schema, "query User { ") == [
            "user",
            "users",
            "search",
            "__typename",
        ]

    def test_unknown_field_yields_nothing(self, schema: Schema):
        assert names(schema, "{ nope { ") == []


class TestMultipleDefinitions:
    """A document can hold several operations and fragments, in any order."""

    def test_operation_after_a_fragment(self, schema: Schema):
        text = "fragment Fields on Post {\n  title\n}\n\nquery GetUser {\n  user { "

        assert names(schema, text) == ["id", "name", "posts", "email", "__typename"]

    def test_fragment_after_an_operation(self, schema: Schema):
        text = "query GetUser {\n  user { id }\n}\n\nfragment Fields on Post {\n  "

        assert names(schema, text) == ["title", "author", "__typename"]

    def test_a_fragments_type_condition_does_not_leak(self, schema: Schema):
        """The type a fragment is `on` only applies within that fragment."""
        text = "fragment Fields on Post {\n  title\n}\n\nquery {\n  "
        context = analyze(text, len(text))

        assert context.base_type is None
        assert names(schema, text) == ["user", "users", "search", "__typename"]

    def test_second_fragment_uses_its_own_type_condition(self, schema: Schema):
        text = "fragment Titles on Post {\n  title\n}\n\nfragment Names on User {\n  "
        context = analyze(text, len(text))

        assert context.base_type == "User"
        assert names(schema, text) == ["id", "name", "posts", "email", "__typename"]

    def test_mutation_after_a_fragment(self, schema: Schema):
        text = "fragment Fields on User {\n  id\n}\n\nmutation {\n  "

        assert names(schema, text) == ["createUser", "__typename"]


class TestFragments:
    def test_type_condition_offers_the_possible_types(self, schema: Schema):
        assert names(schema, "{ search { ... on ") == ["User", "Post"]

    def test_fields_within_an_inline_fragment(self, schema: Schema):
        assert names(schema, "{ search { ... on Post { ") == [
            "title",
            "author",
            "__typename",
        ]

    def test_union_only_offers_typename(self, schema: Schema):
        assert names(schema, "{ search { ") == ["__typename"]


class TestArguments:
    def test_argument_names(self, schema: Schema):
        assert names(schema, "{ user(") == ["id", "role"]

    def test_argument_names_after_an_earlier_argument(self, schema: Schema):
        assert names(schema, '{ user(id: "1", ') == ["id", "role"]

    def test_argument_names_of_a_nested_field(self, schema: Schema):
        assert names(schema, "{ user { posts(") == ["first"]

    def test_enum_values_in_argument_position(self, schema: Schema):
        assert names(schema, "{ user(role: ") == ["ADMIN", "MEMBER"]

    def test_input_object_fields_in_argument_position(self, schema: Schema):
        assert names(schema, "mutation { createUser(input: ") == [
            "name",
            "role",
            "profile",
        ]

    def test_fields_inside_an_input_object_literal(self, schema: Schema):
        assert names(schema, "mutation { createUser(input: {") == [
            "name",
            "role",
            "profile",
        ]

    def test_enum_values_inside_an_input_object_literal(self, schema: Schema):
        assert names(schema, "mutation { createUser(input: {role: ") == [
            "ADMIN",
            "MEMBER",
        ]

    def test_nested_input_object_literal(self, schema: Schema):
        assert names(schema, "mutation { createUser(input: {profile: {") == ["city"]

    def test_selection_set_after_an_input_object_literal(self, schema: Schema):
        assert names(schema, 'mutation { createUser(input: {name: "x"}) { ') == [
            "id",
            "name",
            "posts",
            "email",
            "__typename",
        ]

    def test_scalar_argument_values_yield_nothing(self, schema: Schema):
        assert names(schema, "{ user(id: ") == []


class TestVariables:
    def test_declared_variables_are_offered(self, schema: Schema):
        text = "query Get($id: ID!, $role: Role) { user(id: $"

        assert names(schema, text) == ["id", "role"]

    def test_partially_typed_variable(self, schema: Schema):
        text = "query Get($id: ID!, $role: Role) { user(id: $ro"
        context = analyze(text, len(text))

        assert context.kind == "variable"
        assert context.prefix == "ro"
        assert names(schema, text) == ["id", "role"]


class TestNonCompletablePositions:
    @pytest.mark.parametrize(
        "text",
        [
            "",
            "query ",
            "query Get($id: ID",
            "{ user { name # a comment ",
            '{ user(id: "abc',
            '{ user(id: """block ',
        ],
    )
    def test_no_completions(self, schema: Schema, text: str):
        assert names(schema, text) == []

    def test_cursor_inside_a_closed_string_still_completes(self, schema: Schema):
        text = '{ user(id: "1") { na'

        assert names(schema, text) == ["id", "name", "posts", "email", "__typename"]


class TestContext:
    def test_prefix_and_start(self, schema: Schema):
        text = "{ user { nam"
        context = analyze(text, len(text))

        assert context.kind == "field"
        assert context.prefix == "nam"
        assert context.start == len(text) - 3
        assert context.path == ("user",)
        assert context.operation == "query"

    def test_cursor_before_the_end_of_the_document(self, schema: Schema):
        text = "{ user { na } }"
        cursor = text.index(" }")

        assert names(schema, text, cursor) == [
            "id",
            "name",
            "posts",
            "email",
            "__typename",
        ]

    def test_multiline_document(self, schema: Schema):
        text = "query {\n  user(id: 1) {\n    po"

        assert names(schema, text) == ["id", "name", "posts", "email", "__typename"]

    def test_operation_kind_is_detected(self, schema: Schema):
        assert analyze("mutation Create { ", 18).operation == "mutation"
        assert analyze("subscription Watch { ", 21).operation == "subscription"
