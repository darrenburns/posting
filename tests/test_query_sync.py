import asyncio
from pathlib import Path

import httpx
import pytest
from textual.lazy import Lazy

from posting.__main__ import make_posting
from posting.collection import Options, QueryParam, RequestModel
from posting.urls import extract_query_pairs
from posting.widgets.key_value import KeyValueEditor
from posting.yaml import Loader, load


async def outgoing_pairs(screen):
    request = screen.build_request_model(Options())
    request.apply_template({})
    async with httpx.AsyncClient() as client:
        return request.to_httpx(client).url.params.multi_items()


@pytest.fixture
def query_app(tmp_path, monkeypatch):
    monkeypatch.setenv(
        "POSTING_CONFIG_FILE",
        str(Path(__file__).parent / "sample-configs" / "general.yaml"),
    )

    def run(check):
        async def exercise_app():
            app = make_posting(collection=tmp_path, env=())
            async with app.run_test() as pilot:
                await pilot.pause()
                # The request panels mount after the initial frame. Wait for
                # their actual readiness, including under parallel test load.
                async with asyncio.timeout(10):
                    while app.query(Lazy):
                        await pilot.pause()
                await check(app.screen, pilot)

        asyncio.run(exercise_app())

    return run


@pytest.mark.parametrize("second_value", ["1", "2"])
def test_query_sync_preserves_duplicates(query_app, second_value):
    async def check(screen, pilot):
        screen.url_input.value = f"http://example.com/?q=1&q={second_value}"
        await pilot.pause()
        assert await outgoing_pairs(screen) == [("q", "1"), ("q", second_value)]
        screen.url_input.value = "http://example.com/?other=updated"
        await pilot.pause()
        assert await outgoing_pairs(screen) == [("other", "updated")]

    query_app(check)


@pytest.mark.parametrize("params", [[], [QueryParam(name="q", value="")]])
def test_query_sync_loads_saved_url(query_app, params):
    async def check(screen, pilot):
        original = RequestModel(url="http://example.com/?q=1", params=params)
        screen.load_request_model(original)
        await pilot.pause()
        assert screen.params_table.to_model() == [QueryParam(name="q", value="1")]
        assert await outgoing_pairs(screen) == [("q", "1")]
        assert original.url == "http://example.com/?q=1"
        assert original.params == params

    query_app(check)


@pytest.mark.parametrize("second_value", ["1", "2"])
def test_query_sync_preserves_disabled_duplicate_on_url_edit(query_app, second_value):
    async def check(screen, pilot):
        screen.url_input.value = f"http://example.com/?q=1&q={second_value}"
        await pilot.pause()
        table = screen.params_table
        table.move_cursor(row=0)
        table.action_toggle_row()
        await pilot.pause()
        assert extract_query_pairs(screen.url_input.value) == [("q", second_value)]
        screen.url_input.value += "#fragment"
        await pilot.pause()
        assert table.to_model() == [
            QueryParam(name="q", value="1", enabled=False),
            QueryParam(name="q", value=second_value),
        ]
        assert await outgoing_pairs(screen) == [("q", second_value)]

    query_app(check)


def test_query_sync_does_not_rewrite_while_typing(query_app):
    async def check(screen, pilot):
        screen.url_input.focus()
        text = "http://example.com/?q=%26&two=hello+world"
        for char in text:
            await pilot.press(char)
            await pilot.pause()
        assert screen.url_input.value == text
        assert await outgoing_pairs(screen) == [("q", "&"), ("two", "hello world")]

    query_app(check)


def test_query_sync_follows_table_edits(query_app):
    async def check(screen, pilot):
        screen.load_request_model(RequestModel(url="http://example.com/?q=1&q=2"))
        await pilot.pause()
        table = screen.params_table
        editor = table.query_ancestor(KeyValueEditor)
        field = editor.key_value_input
        field.key_input.value = "blank"
        field.value_input.value = ""
        field.value_input.focus()
        await pilot.press("enter")
        await pilot.pause()
        assert extract_query_pairs(screen.url_input.value) == [
            ("q", "1"), ("q", "2"), ("blank", "")
        ]
        table.move_cursor(row=1)
        editor.action_edit_row("value")
        field.value_input.value = "two & more"
        await pilot.press("enter")
        await pilot.pause()
        assert await outgoing_pairs(screen) == [
            ("q", "1"), ("q", "two & more"), ("blank", "")
        ]
        table.move_cursor(row=1)
        editor.action_edit_row("value")
        field.value_input.value = "cancel me"
        await pilot.press("escape")
        await pilot.pause()
        assert (await outgoing_pairs(screen))[1] == ("q", "two & more")
        table.action_toggle_row()
        await pilot.pause()
        assert await outgoing_pairs(screen) == [("q", "1"), ("blank", "")]
        table.action_toggle_row()
        await pilot.pause()
        assert (await outgoing_pairs(screen))[1] == ("q", "two & more")
        table.action_remove_row()
        await pilot.pause()
        assert extract_query_pairs(screen.url_input.value) == [("q", "1"), ("blank", "")]
        table.move_cursor(row=0)
        table.action_remove_row()
        await pilot.pause()
        table.action_remove_row()
        await pilot.pause()
        assert screen.url_input.value == "http://example.com/"
        assert await outgoing_pairs(screen) == []

    query_app(check)


def test_query_sync_survives_save_reload(query_app, tmp_path):
    async def check(screen, pilot):
        screen.url_input.value = "http://example.com/?q=1&q=2"
        await pilot.pause()
        table = screen.params_table
        table.move_cursor(row=0)
        table.action_toggle_row()
        await pilot.pause()
        model = screen.build_request_model(Options())
        saved = tmp_path / "saved.posting.yaml"
        model.save_to_disk(saved)
        loaded = RequestModel(**load(saved.read_text(), Loader=Loader))
        screen.load_request_model(RequestModel(url="http://other.example/"))
        await pilot.pause()
        screen.load_request_model(loaded)
        await pilot.pause()
        assert table.to_model() == model.params
        assert await outgoing_pairs(screen) == [("q", "2")]

    query_app(check)


def test_query_sync_preserves_variables_and_reserved_characters(query_app):
    async def check(screen, pilot):
        screen.url_input.value = "${BASE_URL}?token=${TOKEN}"
        await pilot.pause()
        model = screen.build_request_model(Options())
        model.apply_template({"BASE_URL": "http://example.com/", "TOKEN": "a&b+c=d"})
        async with httpx.AsyncClient() as client:
            assert model.to_httpx(client).url.params.multi_items() == [("token", "a&b+c=d")]

    query_app(check)
