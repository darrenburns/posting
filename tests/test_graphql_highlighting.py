from posting.graphql.highlighting import highlight_lines


def styles(document: str) -> list[tuple[str, str]]:
    """The text and style of every highlighted token in a document."""
    lines = document.splitlines()
    return [
        (line[start:end] if end is not None else line[start:], style)
        for index, highlights in highlight_lines(lines).items()
        for line in [lines[index]]
        for start, end, style in highlights
    ]


class TestTokens:
    def test_keywords_and_operation_name(self):
        assert styles("query GetUser {")[:2] == [
            ("query", "keyword"),
            ("GetUser", "function"),
        ]

    def test_operation_keywords(self):
        for keyword in ("query", "mutation", "subscription", "fragment"):
            assert styles(f"{keyword} Name {{")[0] == (keyword, "keyword")

    def test_fields(self):
        assert styles("{ id name }") == [
            ("{", "punctuation.bracket"),
            ("id", "json.label"),
            ("name", "json.label"),
            ("}", "punctuation.bracket"),
        ]

    def test_arguments_and_their_values(self):
        assert styles('user(id: "1", role: ADMIN, first: 10, live: true)') == [
            ("user", "json.label"),
            ("(", "punctuation.bracket"),
            ("id", "css.property"),
            (":", "punctuation.delimiter"),
            ('"1"', "string"),
            (",", "punctuation.delimiter"),
            ("role", "css.property"),
            (":", "punctuation.delimiter"),
            ("ADMIN", "constant.builtin"),
            (",", "punctuation.delimiter"),
            ("first", "css.property"),
            (":", "punctuation.delimiter"),
            ("10", "number"),
            (",", "punctuation.delimiter"),
            ("live", "css.property"),
            (":", "punctuation.delimiter"),
            ("true", "boolean"),
            (")", "punctuation.bracket"),
        ]

    def test_variable_definitions(self):
        assert styles("query Q($id: ID!, $role: Role = ADMIN) {")[3:] == [
            ("$id", "type.builtin"),
            (":", "punctuation.delimiter"),
            ("ID", "type"),
            ("!", "operator"),
            (",", "punctuation.delimiter"),
            ("$role", "type.builtin"),
            (":", "punctuation.delimiter"),
            ("Role", "type"),
            ("=", "operator"),
            ("ADMIN", "constant.builtin"),
            (")", "punctuation.bracket"),
            ("{", "punctuation.bracket"),
        ]

    def test_variable_usage(self):
        assert ("$id", "type.builtin") in styles("{ user(id: $id) { name } }")

    def test_aliases(self):
        assert styles("{ fullName: name }")[1:4] == [
            ("fullName", "css.property"),
            (":", "punctuation.delimiter"),
            ("name", "json.label"),
        ]

    def test_inline_fragments(self):
        assert styles("{ ... on User { id } }")[1:4] == [
            ("...", "operator"),
            ("on", "keyword"),
            ("User", "type"),
        ]

    def test_directives(self):
        assert ("@include", "function") in styles("{ name @include(if: $flag) }")

    def test_list_types(self):
        assert ("String", "type") in styles("query Q($tags: [String!]) {")

    def test_floats(self):
        assert ("2.5e3", "number") in styles("{ url(size: 2.5e3) }")


class TestComments:
    def test_comment_to_end_of_line(self):
        assert styles("{ id # a comment") == [
            ("{", "punctuation.bracket"),
            ("id", "json.label"),
            ("# a comment", "comment"),
        ]

    def test_a_hash_inside_a_string_is_not_a_comment(self):
        assert ('"a # b"', "string") in styles('{ user(id: "a # b") }')


class TestStrings:
    def test_block_strings_span_lines(self):
        document = '{\n  """\n  a block # string\n  """\n  id\n}'

        assert [style for _, style in styles(document)].count("string") == 3
        assert ("id", "json.label") in styles(document)

    def test_unterminated_block_string_runs_to_the_end(self):
        highlights = highlight_lines(['{ """oops', "  still a string"])

        assert highlights[0][-1] == (2, None, "string")
        assert highlights[1] == [(0, None, "string")]

    def test_unterminated_string(self):
        assert styles('{ user(id: "oops')[-1] == ('"oops', "string")

    def test_escapes_inside_strings(self):
        assert ('"c\\"d"', "string") in styles('{ a(b: "c\\"d") }')


class TestDocuments:
    def test_empty_document(self):
        assert highlight_lines([]) == {}
        assert highlight_lines([""]) == {}

    def test_only_lines_with_highlights_are_returned(self):
        highlights = highlight_lines(["{", "", "  id", "}"])

        assert sorted(highlights) == [0, 2, 3]

    def test_highlights_are_within_their_line(self):
        lines = ["query GetUser {", "  user(id: 1) { name }", "}"]

        for index, line_highlights in highlight_lines(lines).items():
            for start, end, _style in line_highlights:
                assert 0 <= start <= len(lines[index])
                assert end is None or start < end <= len(lines[index])

    def test_state_carries_across_lines(self):
        # The type condition is on the line after `on`.
        assert ("User", "type") in styles("{ ...\n on\n User { id } }")
