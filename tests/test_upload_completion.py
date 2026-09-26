import pytest
from textual.widgets import Select, TabbedContent
from posting.__main__ import make_posting
from posting.variables import VARIABLES
from posting.widgets.file_autocomplete import FormValueInput
from posting.widgets.request.form_editor import FormTable
from test_snapshots import use_config


@use_config("general.yaml")
@pytest.mark.parametrize(
    "action", ["dropdown", "enter", "tab", "escape", "directory", "literal", "variable"]
)
def test_upload_completion(tmp_path, snap_compare, action):
    (tmp_path / "photo.txt").write_text("upload")
    (tmp_path / "nested").mkdir()
    (tmp_path / "nested" / "note.txt").write_text("nested upload")
    app = make_posting(collection=tmp_path)

    async def run_before(pilot):
        await pilot.pause()
        VARIABLES.set({"TOKEN": "test"})
        app.screen.query_one(
            "RequestEditorTabbedContent", TabbedContent
        ).active = "body-pane"
        app.screen.query_one(
            "#request-body-type-select", Select
        ).value = "form-body-editor"
        app.screen.collection_browser.border_subtitle = "collection"
        field = app.screen.query_one(FormValueInput)
        field.focus()
        await pilot.pause()
        prefix = {"directory": "@ne", "literal": "@@photo", "variable": "$TO"}.get(
            action, "@pho"
        )
        await pilot.press(*prefix)
        await pilot.pause()
        if action == "literal":
            assert not field.auto_complete.display
        else:
            assert field.auto_complete.display
        if action in {"enter", "tab"}:
            await pilot.press(action)
            assert field.value == "@photo.txt"
            assert app.screen.query_one(FormTable).row_count == 0
        elif action == "directory":
            await pilot.press("enter")
            await pilot.pause()
            assert field.value == "@nested/"
            assert field.auto_complete.display
        elif action == "escape":
            await pilot.press("escape")
            assert field.value == "@pho"
            assert not field.auto_complete.display
        elif action == "variable":
            await pilot.press("enter")
            assert field.value == "$TOKEN"

    assert snap_compare(app, run_before=run_before, terminal_size=(120, 40))
