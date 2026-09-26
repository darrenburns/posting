import asyncio
from pathlib import Path
from unittest.mock import Mock

import pytest

from posting.__main__ import make_posting
from posting import variables
from posting.scripts import Posting as ScriptAPI
from posting.widgets.key_value import KeyValueEditor
from posting.widgets.variables_editor import VariablesEditor, VariablesTable
from posting.widgets.variables_modal import VariablesModal


@pytest.fixture
def app(tmp_path, monkeypatch):
    monkeypatch.setattr(variables, "VARIABLES", variables.SharedVariables())
    monkeypatch.setenv(
        "POSTING_CONFIG_FILE",
        str(Path(__file__).parent / "sample-configs" / "general.yaml"),
    )
    env = tmp_path / "variables.env"
    env.write_text("")
    return make_posting(tmp_path, env=(str(env),))


def test_add_first_variable_and_replace_duplicate_name(app):
    async def run():
        async with app.run_test() as pilot:
            await pilot.press("ctrl+shift+v")
            editor = app.screen.query_one(VariablesEditor)
            assert isinstance(editor, KeyValueEditor)
            assert editor.table.row_count == 0
            await pilot.press(*"ITEM_ID", "tab", *"101", "enter")
            await pilot.pause()
            assert variables.get_variables()["ITEM_ID"] == "101"
            assert editor.table.row_count == 1
            await pilot.press("a", *"ITEM_ID", "tab", *"202", "enter")
            await pilot.pause()
            assert variables.get_variables()["ITEM_ID"] == "202"
            assert editor.table.row_count == 1
            assert app.environment_files[0].read_text() == ""
            await pilot.press("v")
            assert editor.key_value_input.key_input.disabled
            assert editor.key_value_input.value_input.value == "202"
            await pilot.press("home", "ctrl+k", *"303", "enter")
            await pilot.pause()
            assert variables.get_variables()["ITEM_ID"] == "303"
            assert not editor.key_value_input.key_input.disabled
            await pilot.press("d")
            await pilot.pause()
            assert editor.table.row_count == 0
            assert "ITEM_ID" not in variables.get_variables()
            editor.table.focus()
            editor.action_edit_row("value")

    asyncio.run(run())


def test_secret_edit_cancel_copy_and_save_use_real_value(app):
    async def run():
        ScriptAPI(app).set_variable("TOKEN", "[bold]real-secret[/bold]")
        app.copy_to_clipboard = Mock()
        async with app.run_test() as pilot:
            await pilot.press("ctrl+shift+v")
            editor = app.screen.query_one(VariablesEditor)
            table = editor.query_one(VariablesTable)
            assert table.get_row("TOKEN")[1].plain == "•" * 12
            await pilot.press("c")
            app.copy_to_clipboard.assert_called_with("[bold]real-secret[/bold]")
            await pilot.press("v")
            assert (
                editor.key_value_input.value_input.value == "[bold]real-secret[/bold]"
            )
            assert editor.key_value_input.value_input.password
            assert editor.key_value_input.key_input.disabled
            await pilot.press("home", "ctrl+k", *"discard", "escape")
            await pilot.pause()
            assert isinstance(app.screen, VariablesModal)
            assert variables.get_variables()["TOKEN"] == "[bold]real-secret[/bold]"
            assert table.get_row("TOKEN")[1].plain == "•" * 12
            await pilot.press("v", "home", "ctrl+k", *"updated", "enter")
            await pilot.pause()
            assert variables.get_variables()["TOKEN"] == "updated"
            assert table.get_row("TOKEN")[1].plain == "•" * 12
            await pilot.press("s")
            await pilot.pause()
            assert table.get_row("TOKEN")[1].plain == "updated"

    asyncio.run(run())


def test_invalid_name_keeps_the_draft_without_adding_a_row(app):
    async def run():
        async with app.run_test() as pilot:
            await pilot.press("ctrl+shift+v", *"bad-name", "tab", *"value", "enter")
            await pilot.pause()
            editor = app.screen.query_one(VariablesEditor)
            assert not variables.get_variables()
            assert editor.table.row_count == 0
            assert editor.key_value_input.key_input.value == "bad-name"
            assert editor.key_value_input.value_input.value == "value"

    asyncio.run(run())


def test_reload_updates_filtered_table_and_keeps_an_edit_draft(app):
    async def run():
        env = app.environment_files[0]
        env.write_text("HOST=old\nOTHER=unrelated\n")
        app.reload_variables()
        async with app.run_test() as pilot:
            await pilot.press("ctrl+shift+v", "slash", *"HOST", "escape")
            editor = app.screen.query_one(VariablesEditor)
            assert editor.table.row_count == 1
            env.write_text("HOST=new\nOTHER=unrelated\n")
            app.reload_variables()
            await pilot.pause()
            assert editor.table.get_row("HOST")[1].plain == "new"
            await pilot.press("v", "home", "ctrl+k", *"draft")
            env.write_text("HOST=newer\nOTHER=unrelated\n")
            app.reload_variables()
            await pilot.pause()
            assert editor.key_value_input.value_input.value == "draft"
            await pilot.press("escape")
            await pilot.pause()
            assert editor.table.get_row("HOST")[1].plain == "newer"

    asyncio.run(run())


def test_undo_preserves_type_and_survives_reopening(app):
    async def run():
        ScriptAPI(app).set_variable("COUNT", 42)
        async with app.run_test() as pilot:
            await pilot.press("ctrl+shift+v", "v", "home", "ctrl+k", *"43", "enter")
            await pilot.press("escape", "ctrl+shift+v", "u")
            await pilot.pause()
            assert ScriptAPI(app).get_variable("COUNT") == 42
            assert variables.get_variables()["COUNT"] == 42
            await pilot.press("ctrl+r")
            await pilot.pause()
            assert ScriptAPI(app).get_variable("COUNT") == "43"

    asyncio.run(run())


def test_revert_uses_the_current_environment_file(app, tmp_path):
    async def run():
        ScriptAPI(app).set_variable("HOST", "override")
        async with app.run_test() as pilot:
            await pilot.press("ctrl+shift+v")
            new_env = tmp_path / "new.env"
            new_env.write_text("HOST=new-file\n")
            app.environment_files = (new_env,)
            await pilot.press("d")
            await pilot.pause()
            assert variables.get_variables()["HOST"] == "new-file"
            table = app.screen.query_one(VariablesTable)
            assert table.get_row("HOST")[2].plain == "env file"
            await pilot.press("u")
            await pilot.pause()
            assert variables.get_variables()["HOST"] == "override"

    asyncio.run(run())


def test_switching_rows_keeps_the_new_row_in_edit_mode(app):
    async def run():
        ScriptAPI(app).set_variable("FIRST", "one")
        ScriptAPI(app).set_variable("SECOND", "two")
        async with app.run_test() as pilot:
            await pilot.press("ctrl+shift+v", "v")
            editor = app.screen.query_one(VariablesEditor)
            editor.table.move_cursor(row=1)
            editor.action_edit_row("value")
            await pilot.pause()
            assert editor.key_value_input.edit_mode
            assert editor.key_value_input.key_input.value == "SECOND"
            assert editor.key_value_input.value_input.value == "two"

    asyncio.run(run())
