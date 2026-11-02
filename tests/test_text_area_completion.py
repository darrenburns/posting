from textual.widgets import TabbedContent, Select
import pytest
from posting.__main__ import make_posting
from posting.variables import VARIABLES
from posting.widgets.request.request_body import RequestBodyTextArea
from test_snapshots import use_config, SAMPLE_COLLECTIONS


@use_config("general.yaml")
@pytest.mark.parametrize(
    "key",
    [
        "dropdown",
        "down",
        "up",
        "enter",
        "tab",
        "escape",
        "escape-down",
        "braces",
        "middle",
        "undo",
        "escaped",
    ],
)
def test_body_variable_completion(key, snap_compare):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())

    async def run_before(pilot):
        await pilot.pause()
        VARIABLES.set({"API_HOST": "https://example.com", "API_TOKEN": "example"})
        app.screen.query_one(
            "RequestEditorTabbedContent", TabbedContent
        ).active = "body-pane"
        app.screen.query_one(
            "#request-body-type-select", Select
        ).value = "text-body-editor"
        area = app.screen.query_one(RequestBodyTextArea)
        area.focus()
        await pilot.pause()
        await pilot.press(*"$API")
        await pilot.pause()
        completion = area.auto_complete
        assert completion.display
        if key in {"down", "up"}:
            await pilot.press(key)
            assert area.text == "$API"
            assert completion.option_list.highlighted == 1
        elif key in {"enter", "tab", "undo"}:
            await pilot.press("enter" if key == "undo" else key)
            assert area.text == "$API_HOST"
            assert not completion.display
            if key == "undo":
                await pilot.press("ctrl+z")
                assert area.text == "$API"
        elif key in {"escape", "escape-down"}:
            await pilot.press("escape")
            assert not completion.display
            if key == "escape-down":
                await pilot.press("down")
                assert not completion.display
            assert area.text == "$API"
        elif key in {"braces", "middle"}:
            area.text = (
                'prefix\n"${API}" suffix'
                if key == "braces"
                else 'prefix\n"$API" suffix'
            )
            area.cursor_location = (1, 6 if key == "braces" else 5)
            await pilot.pause()
            await pilot.press("x", "backspace")
            await pilot.press("enter")
            assert area.text == (
                'prefix\n"${API_HOST}" suffix'
                if key == "braces"
                else 'prefix\n"$API_HOST" suffix'
            )
        elif key == "escaped":
            area.text = ""
            await pilot.press(*"$$API")
            assert not completion.display

    assert snap_compare(app, run_before=run_before, terminal_size=(120, 40))


@use_config("general.yaml")
def test_description_variable_completion(snap_compare):
    app = make_posting(collection=SAMPLE_COLLECTIONS, env=())

    async def run_before(pilot):
        await pilot.pause()
        VARIABLES.set({"API_HOST": "https://example.com", "API_TOKEN": "example"})
        app.screen.query_one("RequestEditorTabbedContent", TabbedContent).active = "info-pane"
        from posting.widgets.variable_text_area import VariableTextArea
        area = app.screen.query_one("#description-textarea", VariableTextArea)
        area.focus()
        await pilot.pause()
        await pilot.press(*"Use $API")
        await pilot.pause()
        assert area.auto_complete.display
        await pilot.press("tab")
        assert area.text == "Use $API_HOST"
        assert area.has_focus
    assert snap_compare(app, run_before=run_before, terminal_size=(120, 40))
