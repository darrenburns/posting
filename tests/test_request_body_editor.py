"""Tests for loading request bodies into the editor.

Every body type goes through the same path when a request is loaded, so
these cover the types which existed before GraphQL was added too.
"""

from pathlib import Path

import pytest

from posting.__main__ import make_posting
from posting.collection import (
    FormItem,
    GraphQLBody,
    RequestBody,
    RequestModel,
)

TESTS_DIR = Path(__file__).parent
SAMPLE_COLLECTIONS = TESTS_DIR / "sample-collections"

pytestmark = pytest.mark.usefixtures("app_environment")

RAW_BODY = RequestBody(content='{"name": "John"}')
FORM_BODY = RequestBody(form_data=[FormItem(name="something", value="123")])
GRAPHQL_BODY = RequestBody(graphql=GraphQLBody(query="query { user { id } }"))


def request_with(body: RequestBody | None) -> RequestModel:
    return RequestModel(
        name="Example", method="POST", url="https://example.com", body=body
    )


@pytest.mark.anyio
@pytest.mark.parametrize(
    "body,expected_type",
    [
        (RAW_BODY, "text-body-editor"),
        (FORM_BODY, "form-body-editor"),
        (GRAPHQL_BODY, "graphql-body-editor"),
        (None, "no-body-label"),
        # A body with nothing in it is no body at all.
        (RequestBody(), "no-body-label"),
        (RequestBody(content=""), "no-body-label"),
    ],
)
async def test_loading_a_body_selects_its_type(
    open_body_tab, body: RequestBody | None, expected_type: str
):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen = await open_body_tab(pilot)

        screen.load_request_model(request_with(body))
        await pilot.pause()

        request_editor = screen.request_editor
        assert request_editor.request_body_type_select.value == expected_type
        assert request_editor.request_body_content_switcher.current == expected_type


@pytest.mark.anyio
async def test_loading_a_raw_body(open_body_tab):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen = await open_body_tab(pilot)

        screen.load_request_model(request_with(RAW_BODY))
        await pilot.pause()

        assert screen.request_body_text_area.text == '{"name": "John"}'
        assert screen.request_body_has_content() is True
        # The editors for the other body types are cleared.
        assert screen.request_editor.form_editor.to_model() == []
        assert screen.request_editor.graphql_editor.has_content is False


@pytest.mark.anyio
async def test_loading_a_form_body(open_body_tab):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen = await open_body_tab(pilot)

        screen.load_request_model(request_with(FORM_BODY))
        await pilot.pause()

        assert screen.request_editor.form_editor.to_model() == [
            FormItem(name="something", value="123")
        ]
        assert screen.request_body_text_area.text == ""
        assert screen.request_editor.graphql_editor.has_content is False


@pytest.mark.anyio
async def test_loading_a_graphql_body(open_body_tab):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen = await open_body_tab(pilot)

        screen.load_request_model(request_with(GRAPHQL_BODY))
        await pilot.pause()

        graphql_editor = screen.request_editor.graphql_editor
        assert graphql_editor.query_text_area.text == "query { user { id } }"
        assert screen.request_body_text_area.text == ""
        assert screen.request_editor.form_editor.to_model() == []


@pytest.mark.anyio
async def test_loading_one_body_after_another(open_body_tab):
    """Loading a request replaces whatever the previous one left behind."""
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen = await open_body_tab(pilot)

        for body in (RAW_BODY, GRAPHQL_BODY, FORM_BODY, None):
            screen.load_request_model(request_with(body))
            await pilot.pause()

        request_editor = screen.request_editor
        assert request_editor.request_body_type_select.value == "no-body-label"
        assert screen.request_body_text_area.text == ""
        assert request_editor.form_editor.to_model() == []
        assert request_editor.graphql_editor.has_content is False
        assert screen.request_body_has_content() is False


@pytest.mark.anyio
async def test_the_body_indicator_follows_the_selected_type(open_body_tab):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen = await open_body_tab(pilot)
        request_editor = screen.request_editor

        # Nothing is selected to begin with, so there's no body.
        assert screen.request_body_has_content() is False

        request_editor.request_body_type_select.value = "text-body-editor"
        screen.request_body_text_area.text = '{"a": 1}'
        await pilot.pause()
        assert screen.request_body_has_content() is True

        # Switching to a body type with nothing in it clears the indicator.
        request_editor.request_body_type_select.value = "no-body-label"
        await pilot.pause()
        assert screen.request_body_has_content() is False

        request_editor.request_body_type_select.value = "text-body-editor"
        await pilot.pause()
        assert screen.request_body_has_content() is True


@pytest.mark.anyio
async def test_a_loaded_body_round_trips_back_into_a_model(open_body_tab):
    """What's loaded into the editor is what gets sent."""
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen = await open_body_tab(pilot)

        for body in (RAW_BODY, FORM_BODY, GRAPHQL_BODY):
            screen.load_request_model(request_with(body))
            await pilot.pause()

            rebuilt = screen.request_editor.to_request_model_args()["body"]
            assert rebuilt.content == body.content
            assert rebuilt.form_data == body.form_data
            assert rebuilt.graphql == body.graphql
