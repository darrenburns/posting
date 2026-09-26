import time
from datetime import datetime, timedelta, timezone
from unittest.mock import patch

import httpx
import pytest
from test_snapshots import SAMPLE_COLLECTIONS, patch_env, use_config
from textual.pilot import Pilot
from textual.widgets import TabbedContent

from posting.__main__ import make_posting
from posting.collection import (
    Auth,
    FormItem,
    Header,
    Options,
    PathParam,
    QueryParam,
    RequestBody,
    RequestModel,
    Scripts,
)
from posting.jump_overlay import JumpOverlay
from posting.messages import HttpResponseReceived
from posting.widgets.collection.history import HistoryBrowser, HistoryList


@pytest.fixture(autouse=True)
def utc_timestamps(monkeypatch):
    with monkeypatch.context() as environment:
        environment.setenv("TZ", "UTC")
        time.tzset()
        yield
    time.tzset()


FIXED_TIME = datetime(2026, 9, 26, 10, 30, tzinfo=timezone.utc)


def saved_response(
    status=200, path="/items", body=b'{"items":[{"id":1,"name":"Notebook"}]}'
):
    response = httpx.Response(
        status,
        content=body,
        headers=[
            ("content-type", "application/json"),
            ("x-request-id", "demo-123"),
            ("set-cookie", "session=demo; Path=/"),
        ],
        request=httpx.Request("GET", f"https://api.example.test{path}"),
    )
    response.elapsed = timedelta(milliseconds=42)
    return response


def saved_request(url="https://api.example.test/items"):
    return RequestModel(
        name="Create item",
        description="Saved configuration",
        method="POST",
        url=url,
        headers=[
            Header(name="Content-Type", value="application/json"),
            Header(name="X-Disabled", value="retained", enabled=False),
        ],
        body=RequestBody(
            content='{ "name": "Notebook" }', content_type="application/json"
        ),
        params=[QueryParam(name="limit", value="5", enabled=False)],
        auth=Auth.bearer_token_auth("$API_TOKEN"),
        options=Options(timeout=12, follow_redirects=False, attach_cookies=False),
        scripts=Scripts(on_request="scripts/prepare.py:prepare"),
    )


def seed_history(screen):
    with patch("posting.history.datetime") as clock:
        clock.now.return_value = FIXED_TIME
        for response in (
            saved_response(),
            saved_response(404, "/missing", b'{"error":"Not found"}'),
        ):
            screen.history_store.record(
                response, saved_request(str(response.request.url))
            )
    screen.query_one(HistoryBrowser).refresh_history()


async def open_history(pilot):
    await pilot.pause()
    await pilot.click("#--content-tab-history-pane")
    await pilot.pause()


@use_config("general.yaml")
class TestHistory:
    @pytest.mark.parametrize("spacing", ["compact", "standard"])
    def test_empty(self, spacing, snap_compare):
        with patch_env("POSTING_SPACING", spacing):
            assert snap_compare(
                make_posting(collection=SAMPLE_COLLECTIONS),
                run_before=open_history,
                terminal_size=(100, 34),
            )

    @pytest.mark.parametrize("spacing", ["compact", "standard"])
    def test_replay_restores_request(self, spacing, snap_compare):
        async def run_before(pilot: Pilot):
            await pilot.pause()
            screen = pilot.app.screen
            screen.url_input.value = "https://draft.example.test/unsent"
            seed_history(screen)
            await open_history(pilot)
            screen.query_one(HistoryList).focus()
            await pilot.press("j", "enter")
            await pilot.pause()
            assert screen.response_area.response.content == saved_response().content
            assert screen.response_area.headers_table.row_count == 4
            assert screen.response_area.cookies_section.table.row_count == 1
            restored = screen.build_request_model(screen.request_options.to_model())
            assert restored.model_dump() == saved_request().model_dump()
            assert screen.collection_tree.currently_open is None
            assert screen.focused is screen.query_one(HistoryList)
            assert not dict(screen.cookies)
            assert len(screen.history_store.entries()) == 2
            assert screen.response_area.tabbed_content.get_tab(
                "response-trace-pane"
            ).disabled
            assert screen.response_area.tabbed_content.get_tab(
                "response-scripts-pane"
            ).disabled

        with patch_env("POSTING_SPACING", spacing):
            assert snap_compare(
                make_posting(collection=SAMPLE_COLLECTIONS),
                run_before=run_before,
                terminal_size=(120, 40),
            )

    def test_send_then_replay_and_send_again(self, snap_compare):
        async def run_before(pilot):
            await pilot.pause()
            screen = pilot.app.screen
            response = saved_response()
            with patch("posting.history.datetime") as clock:
                clock.now.return_value = FIXED_TIME
                screen.post_message(HttpResponseReceived(response, saved_request()))
                await pilot.pause()
            assert len(screen.history_store.entries()) == 1
            await open_history(pilot)
            await pilot.click("#history-list", offset=(2, 1))
            await pilot.pause()
            assert "History" in screen.response_area.border_title
            screen.post_message(
                HttpResponseReceived(saved_response(201, "/created"), saved_request())
            )
            await pilot.pause()
            assert screen.response_area.history_timestamp is None
            assert "History" not in screen.response_area.border_title
            assert not screen.response_area.tabbed_content.get_tab(
                "response-trace-pane"
            ).disabled
            assert len(screen.history_store.entries()) == 2
            # Return to Collections; the request tree is still usable.
            await pilot.click("#--content-tab-collections-pane")
            assert (
                screen.query_one("#sidebar-tabs", TabbedContent).active
                == "collections-pane"
            )

        assert snap_compare(
            make_posting(collection=SAMPLE_COLLECTIONS),
            run_before=run_before,
            terminal_size=(100, 34),
        )

    def test_clear_confirmation_and_delete(self, snap_compare):
        async def run_before(pilot):
            await pilot.pause()
            screen = pilot.app.screen
            seed_history(screen)
            await open_history(pilot)
            screen.query_one(HistoryList).focus()
            await pilot.press("backspace")
            assert len(screen.history_store.entries()) == 1
            await pilot.press("ctrl+backspace")
            await pilot.press("escape")
            assert len(screen.history_store.entries()) == 1
            await pilot.press("ctrl+backspace")
            await pilot.press("y")
            await pilot.pause()
            assert screen.history_store.entries() == []

        assert snap_compare(
            make_posting(collection=SAMPLE_COLLECTIONS),
            run_before=run_before,
            terminal_size=(100, 34),
        )

    def test_persisted_history_on_startup(self, tmp_path, snap_compare):
        from posting.history import HistoryStore

        store = HistoryStore(SAMPLE_COLLECTIONS)
        with patch("posting.history.datetime") as clock:
            clock.now.return_value = FIXED_TIME
            store.record(saved_response(), saved_request())

        async def run_before(pilot):
            await pilot.pause()
            await open_history(pilot)
            await pilot.click("#history-list", offset=(2, 1))
            await pilot.pause()
            assert (
                pilot.app.screen.response_area.response.content
                == saved_response().content
            )
            assert pilot.app.screen.url_input.value == saved_request().url
            assert (
                pilot.app.screen.request_scripts.to_model() == saved_request().scripts
            )

        assert snap_compare(
            make_posting(collection=SAMPLE_COLLECTIONS),
            run_before=run_before,
            terminal_size=(100, 34),
        )

    @patch_env("POSTING_HISTORY__ENABLED", "false")
    def test_disabled(self, snap_compare):
        async def run_before(pilot):
            await pilot.pause()
            screen = pilot.app.screen
            screen.post_message(HttpResponseReceived(saved_response(), saved_request()))
            await pilot.pause()
            assert not screen.history_store.path.exists()
            await open_history(pilot)

        assert snap_compare(
            make_posting(collection=SAMPLE_COLLECTIONS),
            run_before=run_before,
            terminal_size=(100, 34),
        )

    def test_clear_dialog(self, snap_compare):
        async def run_before(pilot):
            await pilot.pause()
            seed_history(pilot.app.screen)
            await open_history(pilot)
            pilot.app.screen.query_one(HistoryList).focus()
            await pilot.press("ctrl+backspace")

        assert snap_compare(
            make_posting(collection=SAMPLE_COLLECTIONS),
            run_before=run_before,
            terminal_size=(100, 34),
        )

    @patch_env("POSTING_COLLECTION_BROWSER__POSITION", "right")
    def test_narrow_right_sidebar(self, snap_compare):
        async def run_before(pilot):
            await pilot.pause()
            screen = pilot.app.screen
            with patch("posting.history.datetime") as clock:
                clock.now.return_value = FIXED_TIME
                screen.history_store.record(
                    saved_response(500, "/a/very/long/path?query=[bold]literal[/bold]"),
                    saved_request(),
                )
            screen.query_one(HistoryBrowser).refresh_history()
            await open_history(pilot)
            screen.query_one(HistoryList).focus()
            await pilot.press("enter")

        assert snap_compare(
            make_posting(collection=SAMPLE_COLLECTIONS),
            run_before=run_before,
            terminal_size=(80, 28),
        )

    def test_storage_failure_keeps_live_response(self, snap_compare):
        async def run_before(pilot):
            await pilot.pause()
            await open_history(pilot)
            screen = pilot.app.screen
            with patch.object(
                screen.history_store,
                "record",
                side_effect=OSError("History is read-only"),
            ):
                screen.post_message(
                    HttpResponseReceived(saved_response(), saved_request())
                )
                await pilot.pause()
            assert screen.response_area.response.content == saved_response().content
            assert screen.history_store.entries() == []

        assert snap_compare(
            make_posting(collection=SAMPLE_COLLECTIONS),
            run_before=run_before,
            terminal_size=(100, 34),
        )

    def test_deleted_elsewhere_does_not_replay(self, snap_compare):
        async def run_before(pilot):
            await pilot.pause()
            screen = pilot.app.screen
            seed_history(screen)
            await open_history(pilot)
            screen.history_store.clear()
            screen.query_one(HistoryList).focus()
            await pilot.press("enter")
            assert screen.response_area.response is None
            assert not screen.query_one(HistoryList).display

        assert snap_compare(
            make_posting(collection=SAMPLE_COLLECTIONS),
            run_before=run_before,
            terminal_size=(100, 34),
        )

    @pytest.mark.parametrize("spacing", ["compact", "standard"])
    def test_jump_mode_and_keyboard_navigation(self, spacing, snap_compare):
        async def run_before(pilot):
            await pilot.pause()
            screen = pilot.app.screen
            seed_history(screen)
            await pilot.press("ctrl+o")
            assert isinstance(pilot.app.screen, JumpOverlay)
            assert "h" not in pilot.app.screen.keys_to_widgets
            await pilot.press("3", "down")
            assert (
                screen.query_one("#sidebar-tabs", TabbedContent).active
                == "history-pane"
            )
            assert screen.focused is screen.query_one(HistoryList)
            # Browsing only highlights; Enter restores both panes.
            screen.url_input.value = "https://draft.example.test"
            await pilot.press("j")
            assert screen.url_input.value == "https://draft.example.test"
            await pilot.press("enter")
            assert screen.url_input.value == saved_request().url
            assert screen.response_area.response.content == saved_response().content
            await pilot.press("g")
            assert screen.query_one(HistoryList).highlighted == 0
            await pilot.press("G")
            assert screen.query_one(HistoryList).highlighted == 1
            await pilot.press("ctrl+l", "ctrl+o", "h")
            assert screen.focused is screen.query_one(HistoryList)
            await pilot.press("ctrl+o", "4", "down")
            assert screen.focused is screen.collection_tree
            await pilot.press("ctrl+o", "3", "down", "ctrl+o")
            assert pilot.app.screen.keys_to_widgets["h"] == "history-list"

        with patch_env("POSTING_SPACING", spacing):
            assert snap_compare(
                make_posting(collection=SAMPLE_COLLECTIONS),
                run_before=run_before,
                terminal_size=(120, 40),
            )

    def test_send_captures_configuration_before_scripts_and_inflight_edits(
        self, snap_compare
    ):
        async def run_before(pilot):
            await pilot.pause()
            await open_history(pilot)
            screen = pilot.app.screen
            request = saved_request("${BASE_URL}/items/:id")
            request.path_params = [PathParam(name="id", value="7")]
            screen.load_request_model(request)
            await pilot.pause()
            expected = screen.build_request_model(
                screen.request_options.to_model()
            ).model_dump()

            def script(path, function, write_logs, model, context):
                assert model.auth.bearer_token.token == "resolved-token"
                model.body.content = '{"changed":"by script"}'
                model.headers.append(Header(name="X-Script", value="ran"))

            async def send(client, request, **kwargs):
                assert str(request.url) == "https://api.example.test/items/7"
                assert request.content == b'{"changed":"by script"}'
                screen.url_input.value = "https://edited.during.request.test"
                result = saved_response()
                result.request = request
                return result

            with (
                patch(
                    "posting.app.get_variables",
                    return_value={
                        "BASE_URL": "https://api.example.test",
                        "API_TOKEN": "resolved-token",
                    },
                ),
                patch.object(screen, "get_and_run_script", side_effect=script),
                patch("httpx.AsyncClient.send", new=send),
                patch("posting.history.datetime") as clock,
            ):
                clock.now.return_value = FIXED_TIME
                await screen.send_request()
                await pilot.pause()
            (entry,) = screen.history_store.entries()
            assert screen.history_store.load(entry.id).request.model_dump() == expected
            with (
                patch.object(screen, "get_and_run_script") as execute,
                patch("httpx.AsyncClient.send") as network,
            ):
                screen.query_one(HistoryList).focus()
                await pilot.press("enter")
                await pilot.pause()
                execute.assert_not_called()
                network.assert_not_called()
            assert (
                screen.build_request_model(
                    screen.request_options.to_model()
                ).model_dump()
                == expected
            )
            assert (
                screen.request_scripts.to_model().on_request
                == "scripts/prepare.py:prepare"
            )

        assert snap_compare(
            make_posting(collection=SAMPLE_COLLECTIONS),
            run_before=run_before,
            terminal_size=(120, 40),
        )

    @pytest.mark.parametrize(
        "body,kind",
        [
            (None, "no-body-label"),
            (RequestBody(content="", content_type=None), "text-body-editor"),
            (RequestBody(content="hello", content_type=None), "text-body-editor"),
            (
                RequestBody(
                    form_data=[], content_type="application/x-www-form-urlencoded"
                ),
                "form-body-editor",
            ),
            (
                RequestBody(
                    form_data=[
                        FormItem(name="a", value="1", enabled=False),
                        FormItem(name="a", value="2"),
                    ],
                    content_type="application/x-www-form-urlencoded",
                ),
                "form-body-editor",
            ),
        ],
    )
    def test_restore_body_clears_previous_content(self, body, kind):
        import asyncio

        async def check():
            app = make_posting(collection=SAMPLE_COLLECTIONS)
            async with app.run_test() as pilot:
                await pilot.pause()
                await open_history(pilot)
                screen = app.screen
                screen.load_request_model(saved_request())
                await pilot.pause()
                request = RequestModel(url="https://api.example.test/items", body=body)
                screen.history_store.record(saved_response(), request)
                screen.query_one(HistoryBrowser).refresh_history()
                screen.query_one(HistoryList).focus()
                await pilot.press("enter")
                await pilot.pause()
                assert screen.request_editor.request_body_type_select.value == kind
                assert screen.request_body_text_area.text == (
                    body.content or "" if body else ""
                )
                assert screen.request_editor.form_editor.to_model() == (
                    body.form_data or [] if body else []
                )
                if kind == "text-body-editor":
                    assert screen.request_editor.text_editor.content_type is None

        asyncio.run(check())

    def test_legacy_entry_keeps_editor_and_explains_limitation(self, snap_compare):
        async def run_before(pilot):
            await pilot.pause()
            screen = pilot.app.screen
            screen.url_input.value = "https://draft.example.test/unsent"
            with patch("posting.history.datetime") as clock:
                clock.now.return_value = FIXED_TIME
                screen.history_store.record(saved_response())
            screen.query_one(HistoryBrowser).refresh_history()
            await open_history(pilot)
            screen.query_one(HistoryList).focus()
            await pilot.press("enter")
            assert screen.url_input.value == "https://draft.example.test/unsent"
            assert screen.response_area.response.content == saved_response().content

        assert snap_compare(
            make_posting(collection=SAMPLE_COLLECTIONS),
            run_before=run_before,
            terminal_size=(100, 34),
        )

    def test_restore_detaches_previously_open_collection_request(self):
        import asyncio

        from posting.widgets.collection.new_request_modal import NewRequestModal

        async def check():
            app = make_posting(collection=SAMPLE_COLLECTIONS)
            async with app.run_test() as pilot:
                await pilot.pause()
                await pilot.press("ctrl+o", "tab", "enter")
                screen = app.screen
                assert screen.collection_tree.currently_open is not None
                original = screen.collection_tree.currently_open.data
                original_data = original.model_dump()
                seed_history(screen)
                await open_history(pilot)
                screen.query_one(HistoryList).focus()
                await pilot.press("enter")
                assert screen.collection_tree.currently_open is None
                assert (
                    screen.build_request_model(screen.request_options.to_model()).path
                    is None
                )
                assert original.model_dump() == original_data
                await pilot.press("ctrl+s")
                assert isinstance(app.screen, NewRequestModal)

        asyncio.run(check())

    def test_missing_script_restores_then_reports_error_on_resend(self):
        import asyncio
        from unittest.mock import AsyncMock

        async def check():
            app = make_posting(collection=SAMPLE_COLLECTIONS)
            async with app.run_test() as pilot:
                await pilot.pause()
                await open_history(pilot)
                screen = app.screen
                request = saved_request()
                request.auth = None
                request.scripts = Scripts(on_request="no-longer-exists.py")
                screen.history_store.record(saved_response(), request)
                screen.query_one(HistoryBrowser).refresh_history()
                screen.query_one(HistoryList).focus()
                with patch.object(screen, "notify", wraps=screen.notify) as notify:
                    await pilot.press("enter")
                    assert screen.request_scripts.to_model() == request.scripts
                    notify.assert_not_called()
                    with patch(
                        "httpx.AsyncClient.send",
                        new=AsyncMock(return_value=saved_response()),
                    ):
                        await screen.send_request()
                        await pilot.pause()
                    assert screen.response_script_output.request_status == "error"
                    assert any(
                        "could not be loaded" in call.kwargs.get("message", "")
                        for call in notify.call_args_list
                    )

        asyncio.run(check())
