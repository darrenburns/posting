"""Tests for the GraphQL editor's behaviour inside the running app."""

from pathlib import Path

import pytest

from posting.__main__ import make_posting
from posting.collection import GraphQLBody, RequestBody, RequestModel
from posting.widgets.request.graphql_operation_modal import GraphQLOperationModal

TESTS_DIR = Path(__file__).parent
SAMPLE_COLLECTIONS = TESTS_DIR / "sample-collections"

pytestmark = pytest.mark.usefixtures("app_environment")


async def show_graphql_editor(pilot, open_body_tab):
    """Show the GraphQL body editor, and return the screen and the editor."""
    screen = await open_body_tab(pilot)
    screen.request_editor.request_body_type_select.value = "graphql-body-editor"
    await pilot.pause()
    return screen, screen.request_editor.graphql_editor


@pytest.mark.anyio
async def test_selecting_graphql_switches_the_method_to_post(open_body_tab):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen = await open_body_tab(pilot)
        assert screen.selected_method == "GET"

        screen.request_editor.request_body_type_select.value = "graphql-body-editor"
        await pilot.pause()

        assert screen.selected_method == "POST"
        assert screen.method_selector.value == "POST"


@pytest.mark.anyio
async def test_selecting_another_body_type_leaves_the_method_alone(open_body_tab):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen = await open_body_tab(pilot)

        screen.request_editor.request_body_type_select.value = "form-body-editor"
        await pilot.pause()

        assert screen.selected_method == "GET"


@pytest.mark.anyio
async def test_loading_a_graphql_request_keeps_its_method(open_body_tab):
    """A request saved as a GET must not be switched to POST when it loads."""
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen = await open_body_tab(pilot)
        request = RequestModel(
            name="Get user",
            method="GET",
            url="https://example.com/graphql",
            body=RequestBody(graphql=GraphQLBody(query="query { user { id } }")),
        )

        screen.load_request_model(request)
        for _ in range(5):
            await pilot.pause()

        assert screen.selected_method == "GET"
        assert screen.method_selector.value == "GET"
        assert (
            screen.request_editor.request_body_type_select.value
            == "graphql-body-editor"
        )
        assert (
            screen.request_editor.graphql_editor.query_text_area.text
            == "query { user { id } }"
        )


@pytest.mark.anyio
async def test_graphql_body_round_trips_through_the_editor(open_body_tab):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen = await open_body_tab(pilot)
        graphql_editor = screen.request_editor.graphql_editor
        screen.request_editor.request_body_type_select.value = "graphql-body-editor"
        await pilot.pause()

        graphql_editor.query_text_area.text = "query GetUser { user { id } }"
        graphql_editor.variables_text_area.text = '{"id": "1"}'
        graphql_editor.operation_name_input.value = "GetUser"
        await pilot.pause()

        body = screen.request_editor.to_request_model_args()["body"]
        assert body.graphql == GraphQLBody(
            query="query GetUser { user { id } }",
            variables='{"id": "1"}',
            operation_name="GetUser",
        )
        assert body.content_type == "application/json"


TWO_OPERATIONS = """\
query Articles {
  articles { items { id } }
}

query Podcasts {
  podcasts { items { id } }
}"""


@pytest.mark.anyio
async def test_choosing_an_operation_before_sending(open_body_tab):
    """A document with two operations must say which one to run."""
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen, graphql_editor = await show_graphql_editor(pilot, open_body_tab)
        graphql_editor.query_text_area.text = TWO_OPERATIONS
        await pilot.pause()

        choice = screen.run_worker(screen.choose_graphql_operation())
        await pilot.pause()

        assert isinstance(app.screen, GraphQLOperationModal)
        await pilot.press("down", "enter")
        await pilot.pause()

        assert await choice.wait() is True
        assert graphql_editor.operation_name_input.value == "Podcasts"


@pytest.mark.anyio
async def test_dismissing_the_prompt_stops_the_request(open_body_tab):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen, graphql_editor = await show_graphql_editor(pilot, open_body_tab)
        graphql_editor.query_text_area.text = TWO_OPERATIONS
        await pilot.pause()

        choice = screen.run_worker(screen.choose_graphql_operation())
        await pilot.pause()
        await pilot.press("escape")
        await pilot.pause()

        assert await choice.wait() is False
        assert graphql_editor.operation_name_input.value == ""


@pytest.mark.anyio
async def test_no_prompt_when_the_operation_is_already_named(open_body_tab):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen, graphql_editor = await show_graphql_editor(pilot, open_body_tab)
        graphql_editor.query_text_area.text = TWO_OPERATIONS
        graphql_editor.operation_name_input.value = "Articles"
        await pilot.pause()

        assert await screen.choose_graphql_operation() is True
        assert isinstance(app.screen, type(screen))


@pytest.mark.anyio
async def test_no_prompt_for_a_single_operation(open_body_tab):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen, graphql_editor = await show_graphql_editor(pilot, open_body_tab)
        graphql_editor.query_text_area.text = "query Articles { articles { id } }"
        await pilot.pause()

        assert await screen.choose_graphql_operation() is True


@pytest.mark.anyio
async def test_no_prompt_for_other_body_types(open_body_tab):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())
    async with app.run_test() as pilot:
        screen, graphql_editor = await show_graphql_editor(pilot, open_body_tab)
        graphql_editor.query_text_area.text = TWO_OPERATIONS
        screen.request_editor.request_body_type_select.value = "text-body-editor"
        await pilot.pause()

        assert await screen.choose_graphql_operation() is True
