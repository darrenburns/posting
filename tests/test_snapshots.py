import os
from pathlib import Path
from unittest import mock
import pytest

from textual.pilot import Pilot
from textual.widgets import Input, TextArea

TEST_DIR = Path(__file__).parent
CONFIG_DIR = TEST_DIR / "sample-configs"
SAMPLE_COLLECTIONS = TEST_DIR / "sample-collections"
POSTING_MAIN = TEST_DIR / "posting_snapshot_app.py"


def use_config(file_name: str):
    return mock.patch.dict(
        os.environ, {"POSTING_CONFIG_FILE": str(CONFIG_DIR / file_name)}
    )


def patch_env(key: str, value: str):
    return mock.patch.dict(os.environ, {key: value})


def no_cursor_blink(pilot: Pilot):
    for input in pilot.app.query(Input):
        input.cursor_blink = False
    for text_area in pilot.app.query(TextArea):
        text_area.cursor_blink = False


@use_config("general.yaml")
class TestJumpMode:
    def test_loads(self, snap_compare):
        """Simple check that ctrl+o enters jump mode."""

        async def run_before(pilot: Pilot):
            no_cursor_blink(pilot)
            await pilot.press("ctrl+o")

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_focus_switch(self, snap_compare):
        """Jump mode can target focusable widgets such as the collection tree."""

        async def run_before(pilot: Pilot):
            no_cursor_blink(pilot)
            await pilot.press("ctrl+o")  # enter jump mode
            await pilot.press("tab")  # target collection tree

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_click_switch(self, snap_compare):
        """Jump mode can target widgets that are not focusable, such as tabs."""

        async def run_before(pilot: Pilot):
            no_cursor_blink(pilot)
            await pilot.press("ctrl+o")  # enter jump mode
            await pilot.press("y")  # target "Options" tab

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
class TestMethodSelection:
    def test_select_post_method(self, snap_compare):
        """Simple check that we can change the method."""

        async def run_before(pilot: Pilot):
            no_cursor_blink(pilot)
            await pilot.press("ctrl+t")
            await pilot.press("p")
            await pilot.press("enter")

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
class TestUrlBar:
    def test_enter_url(self, snap_compare) -> None:
        """Check that we can enter a URL into the URL bar."""

        async def run_before(pilot: Pilot) -> None:
            no_cursor_blink(pilot)
            await pilot.press("ctrl+l")  # Focus the URL bar
            await pilot.press(*"https://example.com/")

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
class TestCommandPalette:
    def test_loads_and_shows_discovery_options(self, snap_compare):
        """Check that the command palette loads."""

        async def run_before(pilot: Pilot):
            no_cursor_blink(pilot)
            await pilot.press("ctrl+p")

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_can_type_to_filter_options(self, snap_compare):
        """Check that we can run a command from the command palette."""

        async def run_before(pilot: Pilot):
            no_cursor_blink(pilot)
            await pilot.press("ctrl+p")
            await pilot.press(*"view")

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_can_run_command__hide_collection_browser(self, snap_compare):
        """Check that we can run a command from the command palette."""

        async def run_before(pilot: Pilot):
            no_cursor_blink(pilot)
            await pilot.press("ctrl+p")
            await pilot.press(*"toggle collection")
            await pilot.press("enter", "enter")

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
@patch_env("POSTING_FOCUS__ON_STARTUP", "collection")
class TestNewRequest:
    def test_dialog_loads_and_can_be_used(self, snap_compare):
        """Check that the new request dialog loads and is prefilled
        with the data based on where the cursor is."""

        async def run_before(pilot: Pilot):
            no_cursor_blink(pilot)
            await pilot.press("J", "J", "ctrl+n")
            await pilot.press(*"foo")
            await pilot.press("tab", "tab")
            await pilot.press(*"bar")

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_new_request_added_to_tree_correctly_and_notification_shown(
        self, snap_compare
    ):
        """Check that the new request is added to the tree correctly
        and that a notification is shown."""

        async def run_before(pilot: Pilot):
            await pilot.press("J", "J", "ctrl+n")
            await pilot.press(*"foo")
            await pilot.press("tab", "tab")
            await pilot.press(*"bar")
            await pilot.press("ctrl+n")

            await pilot.pause()
            # Check the file exists
            new_request_file = (
                SAMPLE_COLLECTIONS / "jsonplaceholder" / "posts" / "foo.posting.yaml"
            )
            assert new_request_file.exists()

            # Check the file content
            assert (
                new_request_file.read_text("utf-8")
                == """\
name: foo
description: bar
"""
            )

            new_request_file.unlink()

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
class TestUserInterfaceShortcuts:
    def test_hide_collection_browser(self, snap_compare):
        """Check that we can hide the collection browser."""

        async def run_before(pilot: Pilot):
            no_cursor_blink(pilot)
            await pilot.press("ctrl+h")

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_expand_request_section(self, snap_compare):
        """Check that we can expand the request section."""

        async def run_before(pilot: Pilot):
            no_cursor_blink(pilot)
            await pilot.press("ctrl+o")  # jump mode
            await pilot.press("q")  # move focus to inside request section
            await pilot.press("ctrl+m")  # expand request section

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_expand_then_reset(self, snap_compare):
        """Check that we can expand the request section and then reset it."""

        async def run_before(pilot: Pilot):
            no_cursor_blink(pilot)
            await pilot.press("ctrl+o")  # jump mode
            await pilot.press("q")  # move focus to inside request section
            await pilot.press("ctrl+m")  # expand request section
            await pilot.press("ctrl+m")  # reset sections

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
@patch_env("POSTING_FOCUS__ON_STARTUP", "collection")
class TestLoadingRequest:
    def test_request_loaded_into_view__headers(self, snap_compare):
        """Check that the request headers are loaded into the view."""

        async def run_before(pilot: Pilot):
            # Navigate to 'get one post' and select it.
            await pilot.press("J", "J", "j")
            await pilot.press("enter")

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 34))

    def test_request_loaded_into_view__body(self, snap_compare):
        """Check that the request body is loaded into the view."""

        async def run_before(pilot: Pilot):
            # Navigate to 'POST one post' and select it.
            await pilot.press("J", "J", "j", "j")
            await pilot.press("enter")
            await pilot.press("ctrl+o", "w")  # jump to 'Body' tab

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 34))

    def test_request_loaded_into_view__query_params(self, snap_compare):
        """Check that the request query params are loaded into the view."""

        async def run_before(pilot: Pilot):
            # Navigate to 'GET comments via query' and select it.
            await pilot.press("J", "J", "J", "j", "j")
            await pilot.press("enter")
            await pilot.press("ctrl+o", "e")  # jump to 'Query Params' tab

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 34))

    def test_request_loaded_into_view__auth(self, snap_compare):
        """Check that the request auth is loaded into the view."""

        async def run_before(pilot: Pilot):
            # Navigate to 'GET comments via query' and select it.
            await pilot.press("j")
            await pilot.press("enter")
            await pilot.press("ctrl+o", "r")  # jump to 'Auth' tab

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 44))

    @pytest.mark.skip(
        reason="info tab contains a path, specific to the host the test runs on"
    )
    def test_request_loaded_into_view__info(self, snap_compare):
        """Check that the request info is loaded into the view."""

        async def run_before(pilot: Pilot):
            await pilot.press("j")
            await pilot.press("enter")
            await pilot.press("ctrl+o", "t")  # jump to 'Info' tab

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 44))

    def test_request_loaded_into_view__options(self, snap_compare):
        """Check that the request options are loaded into the view."""

        async def run_before(pilot: Pilot):
            await pilot.press("j")
            await pilot.press("enter")
            await pilot.press("ctrl+o", "y")  # jump to 'Options' tab

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 44))


@use_config("general.yaml")
class TestHelpScreen:
    def test_help_screen_appears(self, snap_compare):
        """Check that the help screen appears."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+question_mark")

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 42))
