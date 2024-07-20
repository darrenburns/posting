import os
from pathlib import Path
from unittest import mock

from textual.pilot import Pilot
from textual.widgets import Input, TextArea

SNAPSHOT_DIR = Path(__file__).parent
CONFIG_DIR = SNAPSHOT_DIR / "sample-configs"
POSTING_MAIN = SNAPSHOT_DIR / "posting_snapshot_app.py"


def use_config(file_name: str):
    return mock.patch.dict(
        os.environ, {"POSTING_CONFIG_FILE": str(CONFIG_DIR / file_name)}
    )


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
            await pilot.press(*"view toggle collection browser")
            await pilot.press("enter", "enter")

        assert snap_compare(POSTING_MAIN, run_before=run_before)
