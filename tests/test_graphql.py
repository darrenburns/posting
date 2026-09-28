import json

import pytest

from posting.collection import GraphQLBody, GraphQLError, RequestBody, RequestModel
from posting.variables import SubstitutionError

QUERY = """\
query GetUser($id: ID!) {
  user(id: $id) {
    name
  }
}"""


class TestGraphQLBody:
    def test_query_is_escaped_into_json_payload(self):
        body = GraphQLBody(query=QUERY)
        content = body.to_content()

        # The content is valid JSON, with the newlines and quotes escaped for us.
        assert json.loads(content) == {"query": QUERY}
        assert "\\n" in content

    def test_variables_are_embedded_as_json(self):
        body = GraphQLBody(query=QUERY, variables='{"id": "1"}')

        assert json.loads(body.to_content()) == {
            "query": QUERY,
            "variables": {"id": "1"},
        }

    def test_operation_name_is_included_when_set(self):
        body = GraphQLBody(query=QUERY, operation_name="GetUser")

        assert json.loads(body.to_content()) == {
            "query": QUERY,
            "operationName": "GetUser",
        }

    def test_blank_variables_and_operation_name_are_omitted(self):
        body = GraphQLBody(query=QUERY, variables="  \n ", operation_name="  ")

        assert json.loads(body.to_content()) == {"query": QUERY}

    def test_invalid_variables_json_raises(self):
        body = GraphQLBody(query=QUERY, variables="{not json}")

        with pytest.raises(GraphQLError):
            body.to_content()

    def test_non_object_variables_raises(self):
        body = GraphQLBody(query=QUERY, variables="[1, 2, 3]")

        with pytest.raises(GraphQLError):
            body.to_content()


class TestRequestBody:
    def test_graphql_body_is_sent_as_content(self):
        body = RequestBody(graphql=GraphQLBody(query=QUERY))

        assert body.to_httpx_args() == {"content": body.graphql.to_content()}

    def test_raw_content_is_unaffected(self):
        body = RequestBody(content='{"foo": "bar"}')

        assert body.to_content() == '{"foo": "bar"}'
        assert body.to_httpx_args() == {"content": '{"foo": "bar"}'}


class TestTemplating:
    def make_request(self, **kwargs: str) -> RequestModel:
        return RequestModel(
            method="POST",
            url="https://example.com/graphql",
            body=RequestBody(graphql=GraphQLBody(**kwargs)),
        )

    def test_graphql_variables_in_query_are_left_alone(self):
        request = self.make_request(query=QUERY)
        request.apply_template({})

        assert request.body.graphql.query == QUERY

    def test_braced_posting_variables_are_substituted_in_query(self):
        request = self.make_request(query="query { user(id: $id) { ${field} } }")
        request.apply_template({"field": "email"})

        assert request.body.graphql.query == "query { user(id: $id) { email } }"

    def test_undefined_braced_variable_raises(self):
        request = self.make_request(query="query { ${nope} }")

        with pytest.raises(SubstitutionError):
            request.apply_template({})

    def test_variables_and_operation_name_are_substituted(self):
        request = self.make_request(
            query=QUERY,
            variables='{"id": "$user_id"}',
            operation_name="$operation",
        )
        request.apply_template({"user_id": "42", "operation": "GetUser"})

        assert request.body.graphql.variables == '{"id": "42"}'
        assert request.body.graphql.operation_name == "GetUser"


class TestCurlExport:
    def test_graphql_body_is_exported_as_json_data(self):
        request = RequestModel(
            method="POST",
            url="https://example.com/graphql",
            body=RequestBody(graphql=GraphQLBody(query=QUERY, variables='{"id": "1"}')),
        )

        curl = request.to_curl()

        assert f"-d '{request.body.to_content()}'" in curl
