import time
from datetime import datetime, timedelta, timezone
from unittest.mock import patch

import httpx
import pytest
from test_snapshots import SAMPLE_COLLECTIONS, patch_env, use_config
from textual.pilot import Pilot
from textual.widgets import TabbedContent

from posting.__main__ import make_posting
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


def seed_history(screen):
    with patch("posting.history.datetime") as clock:
        clock.now.return_value = FIXED_TIME
        for response in (
            saved_response(),
            saved_response(404, "/missing", b'{"error":"Not found"}'),
        ):
            screen.history_store.record(response)
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
    def test_replay_preserves_editor(self, spacing, snap_compare):
        async def run_before(pilot: Pilot):
            await pilot.pause()
            screen = pilot.app.screen
            screen.url_input.value = "https://draft.example.test/unsent"
            seed_history(screen)
            await open_history(pilot)
            before = screen.build_request_model(screen.request_options.to_model())
            screen.query_one(HistoryList).focus()
            await pilot.press("j", "enter")
            await pilot.pause()
            assert screen.response_area.response.content == saved_response().content
            assert screen.response_area.headers_table.row_count == 4
            assert screen.response_area.cookies_section.table.row_count == 1
            assert (
                screen.build_request_model(screen.request_options.to_model()) == before
            )
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
                screen.post_message(HttpResponseReceived(response))
                await pilot.pause()
            assert len(screen.history_store.entries()) == 1
            await open_history(pilot)
            await pilot.click("#history-list", offset=(2, 1))
            await pilot.pause()
            assert "History" in screen.response_area.border_title
            screen.post_message(HttpResponseReceived(saved_response(201, "/created")))
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
            store.record(saved_response())

        async def run_before(pilot):
            await pilot.pause()
            await open_history(pilot)
            await pilot.click("#history-list", offset=(2, 1))
            await pilot.pause()
            assert (
                pilot.app.screen.response_area.response.content
                == saved_response().content
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
            screen.post_message(HttpResponseReceived(saved_response()))
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
                    saved_response(500, "/a/very/long/path?query=[bold]literal[/bold]")
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
                screen.post_message(HttpResponseReceived(saved_response()))
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
