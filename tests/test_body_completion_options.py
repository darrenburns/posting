import asyncio

import pytest
from test_snapshots import use_config
from textual.widgets import Checkbox, TabbedContent

from posting.__main__ import make_posting
from posting.collection import FormItem, Options, RequestBody, RequestModel
from posting.variables import VARIABLES
from posting.widgets.request.form_editor import FormVariableInput
from posting.widgets.request.request_body import RequestBodyTextArea


@use_config("general.yaml")
@pytest.mark.parametrize("body_type", ["text-body-editor", "form-body-editor"])
def test_body_completion_follows_substitution_option(body_type, snap_compare, tmp_path):
    app = make_posting(collection=tmp_path)

    async def run_before(pilot):
        await pilot.pause()
        VARIABLES.set({"ITEM_ID": "999"})
        screen = app.screen
        tabs = screen.query_one("RequestEditorTabbedContent", TabbedContent)
        tabs.active = "body-pane"
        async with asyncio.timeout(10):
            while not screen.query(RequestBodyTextArea):
                await pilot.pause()
        screen.request_editor.request_body_type_select.value = body_type
        if body_type == "text-body-editor":
            editor = screen.query_one(RequestBodyTextArea)
        else:
            editor = screen.query_one("FormEditor .value-input", FormVariableInput)

        async def type_prefix():
            tabs.active = "body-pane"
            screen.request_editor.request_body_type_select.value = body_type
            if isinstance(editor, RequestBodyTextArea):
                editor.text = ""
            else:
                editor.value = ""
            editor.focus()
            await pilot.press(*"$IT")
            await pilot.pause()

        await type_prefix()
        assert editor.auto_complete.display
        await pilot.press("tab")
        expression = (
            editor.text if isinstance(editor, RequestBodyTextArea) else editor.value
        )
        assert expression == "$ITEM_ID"
        # Default requests resolve expressions inserted by the dropdown.
        body = (
            RequestBody(content=expression)
            if body_type == "text-body-editor"
            else RequestBody(form_data=[FormItem(name="item", value=expression)])
        )
        request = RequestModel(body=body)
        request.apply_template({"ITEM_ID": "999"})
        if body_type == "text-body-editor":
            assert request.body.content == "999"
        else:
            assert request.body.form_data[0].value == "999"

        tabs.active = "options-pane"
        async with asyncio.timeout(10):
            while not screen.query("#substitute-body-variables"):
                await pilot.pause()
        checkbox = screen.query_one("#substitute-body-variables", Checkbox)
        assert checkbox.value is True
        checkbox.focus()
        await pilot.press("space")
        await type_prefix()
        assert not editor.auto_complete.display

        tabs.active = "options-pane"
        checkbox.focus()
        await pilot.press("space")
        await type_prefix()
        assert editor.auto_complete.display
        await pilot.press("escape")

        # Saved requests and History use this same loading path.
        screen.load_request_model(
            RequestModel(options=Options(substitute_body_variables=False))
        )
        await pilot.pause()
        await type_prefix()
        assert not editor.auto_complete.display

        # The body option must not disable URL completion.
        screen.url_input.value = ""
        screen.url_input.focus()
        await pilot.press(*"$IT")
        await pilot.pause()
        assert screen.url_bar.auto_complete.display
        await pilot.press("escape")
        await type_prefix()
        assert not editor.auto_complete.display
        screen.collection_browser.border_subtitle = "collection"

    assert snap_compare(app, run_before=run_before, terminal_size=(120, 40))
