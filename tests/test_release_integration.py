"""Interactions between the features included in Posting 2.11."""

import asyncio
from datetime import timedelta
from unittest.mock import patch

import httpx
from test_snapshots import use_config
from textual.widgets import Checkbox, TabbedContent

from posting.__main__ import make_posting
from posting.collection import Header, Options, RequestBody, RequestModel
from posting.environments import switch_environment
from posting.variables import get_variables
from posting.widgets.collection.history import HistoryList
from posting.widgets.request.request_body import RequestBodyTextArea
from posting.widgets.variables_editor import VariablesEditor


@use_config("general.yaml")
def test_session_override_survives_switch_and_completion_uses_current_values(tmp_path):
    first = tmp_path / "development.env"
    second = tmp_path / "staging.env"
    first.write_text("ITEM_ID=101\nDEV_ONLY=yes\n")
    second.write_text("ITEM_ID=202\nSTAGING_ONLY=yes\n")
    app = make_posting(collection=tmp_path, env=(str(first),))

    async def check():
        async with app.run_test() as pilot:
            await pilot.pause()
            await pilot.press("ctrl+shift+v")
            await pilot.pause()
            editor = app.screen.query_one(VariablesEditor)
            editor.action_add_variable()
            editor.key_value_input.key_input.value = "ITEM_ID"
            editor.key_value_input.value_input.value = "999"
            editor.key_value_input.value_input.focus()
            await pilot.press("enter")
            assert get_variables()["ITEM_ID"] == "999"
            assert switch_environment(app, (second,))
            await pilot.pause()
            assert get_variables() == {"ITEM_ID": "999", "STAGING_ONLY": "yes"}
            editor.action_undo()
            await pilot.pause()
            assert get_variables()["ITEM_ID"] == "202"
            await pilot.press("escape")
            screen = app.screen
            screen.query_one(
                "RequestEditorTabbedContent", TabbedContent
            ).active = "body-pane"
            screen.request_editor.request_body_type_select.value = "text-body-editor"
            area = screen.query_one(RequestBodyTextArea)
            area.focus()
            await pilot.press(*"$STAGING")
            await pilot.pause()
            assert area.auto_complete.display
            await pilot.press("enter")
            assert area.text == "$STAGING_ONLY"
            assert first.read_text() == "ITEM_ID=101\nDEV_ONLY=yes\n"
            assert second.read_text() == "ITEM_ID=202\nSTAGING_ONLY=yes\n"

    asyncio.run(check())


@use_config("general.yaml")
def test_history_retains_literal_body_option_and_resends_using_new_environment(
    tmp_path,
):
    first = tmp_path / "development.env"
    second = tmp_path / "staging.env"
    first.write_text("API_URL=https://development.example.test\nMODE=development\n")
    second.write_text("API_URL=https://staging.example.test\nMODE=staging\n")
    app = make_posting(collection=tmp_path, env=(str(first),))
    body = '{"$schema":"example","value":"${MODE}","price":"$5"}'
    request = RequestModel(
        url="${API_URL}/items",
        method="POST",
        headers=[Header(name="X-Mode", value="${MODE}")],
        body=RequestBody(content=body, content_type="application/json"),
        options=Options(substitute_body_variables=False),
    )
    sent = []

    async def send(client, request, **kwargs):
        sent.append(request)
        response = httpx.Response(201, content=b'{"ok":true}', request=request)
        response.elapsed = timedelta(milliseconds=10)
        return response

    async def check():
        async with app.run_test() as pilot:
            await pilot.pause()
            screen = app.screen
            screen.load_request_model(request)
            await pilot.pause()
            with patch("httpx.AsyncClient.send", new=send):
                await screen.send_request()
                await pilot.pause()
                assert len(sent) == 1
                (entry,) = screen.history_store.entries()
                assert switch_environment(app, (second,))
                screen.load_request_model(
                    RequestModel(url="https://draft.example.test")
                )
                await pilot.pause()
                assert screen.request_options.to_model().substitute_body_variables
                await pilot.click("#--content-tab-history-pane")
                screen.query_one(HistoryList).focus()
                await pilot.press("enter")
                await pilot.pause()
                assert len(sent) == 1
                assert not screen.query_one(
                    "#substitute-body-variables", Checkbox
                ).value
                assert screen.request_body_text_area.text == body
                assert screen.url_input.value == "${API_URL}/items"
                await screen.send_request()
                await pilot.pause()
            assert [str(item.url) for item in sent] == [
                "https://development.example.test/items",
                "https://staging.example.test/items",
            ]
            assert [item.headers["X-Mode"] for item in sent] == [
                "development",
                "staging",
            ]
            assert all(item.content.decode() == body for item in sent)
            archived = screen.history_store.load(entry.id).request
            assert archived.body.content == body
            assert not archived.options.substitute_body_variables

    asyncio.run(check())
