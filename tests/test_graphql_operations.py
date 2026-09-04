import pytest

from posting.graphql.operations import (
    Operation,
    find_operations,
    operations_to_choose,
)

TWO_OPERATIONS = """\
query Articles {
  articles {
    items {
      externalIdentifiers {
        rumbleId
      }
    }
  }
}

query Podcasts {
  podcasts {
    items {
      id
    }
  }
}"""


class TestFindOperations:
    def test_two_operations(self):
        assert find_operations(TWO_OPERATIONS) == [
            Operation("query", "Articles"),
            Operation("query", "Podcasts"),
        ]

    def test_a_single_operation(self):
        assert find_operations("query One { a }") == [Operation("query", "One")]

    def test_operation_kinds(self):
        document = "query A { a }\nmutation B { b }\nsubscription C { c }"

        assert find_operations(document) == [
            Operation("query", "A"),
            Operation("mutation", "B"),
            Operation("subscription", "C"),
        ]

    def test_an_anonymous_operation(self):
        assert find_operations("{ a }") == [Operation("query", "")]

    def test_an_anonymous_operation_with_variables(self):
        assert find_operations("query($id: ID!) { user(id: $id) { id } }") == [
            Operation("query", "")
        ]

    def test_variable_definitions_are_not_mistaken_for_a_name(self):
        assert find_operations("query Get($id: ID!) { user(id: $id) { id } }") == [
            Operation("query", "Get")
        ]

    def test_fragments_are_not_operations(self):
        document = (
            "fragment Fields on User {\n  id\n}\n\nquery Q {\n  user { ...Fields }\n}"
        )

        assert find_operations(document) == [Operation("query", "Q")]

    def test_a_document_of_only_fragments(self):
        assert find_operations("fragment Fields on User { id }") == []

    def test_nested_braces_do_not_start_an_operation(self):
        document = 'mutation M { create(input: {name: "x"}) { id } }'

        assert find_operations(document) == [Operation("mutation", "M")]

    def test_keywords_inside_strings_and_comments_are_ignored(self):
        document = 'query A {\n  a(text: "query B")  # query C\n}'

        assert find_operations(document) == [Operation("query", "A")]

    def test_an_empty_document(self):
        assert find_operations("") == []
        assert find_operations("   \n  ") == []

    def test_an_unfinished_operation(self):
        assert find_operations("query Half {") == [Operation("query", "Half")]

    def test_string_representation(self):
        assert str(Operation("mutation", "Create")) == "mutation Create"
        assert str(Operation("query", "")) == "query"


class TestOperationsToChoose:
    def test_a_choice_is_needed_for_two_operations(self):
        assert operations_to_choose(TWO_OPERATIONS) == [
            Operation("query", "Articles"),
            Operation("query", "Podcasts"),
        ]

    def test_no_choice_for_a_single_operation(self):
        assert operations_to_choose("query One { a }") == []

    def test_no_choice_when_an_operation_is_already_named(self):
        assert operations_to_choose(TWO_OPERATIONS, "Podcasts") == []
        assert operations_to_choose(TWO_OPERATIONS, "  Articles  ") == []

    def test_a_choice_is_needed_when_the_name_matches_nothing(self):
        """The name may be left over from an operation which has been renamed."""
        assert len(operations_to_choose(TWO_OPERATIONS, "Gone")) == 2

    def test_no_choice_for_an_empty_document(self):
        assert operations_to_choose("") == []

    def test_no_choice_when_none_of_the_operations_are_named(self):
        """An anonymous operation can't be chosen by name."""
        assert operations_to_choose("{ a }\n\n{ b }") == []

    def test_only_named_operations_are_offered(self):
        document = "{ a }\n\nquery Named { b }\n\nquery Other { c }"

        assert operations_to_choose(document) == [
            Operation("query", "Named"),
            Operation("query", "Other"),
        ]

    @pytest.mark.parametrize("name", ["", "   "])
    def test_a_blank_name_is_no_name(self, name: str):
        assert len(operations_to_choose(TWO_OPERATIONS, name)) == 2
