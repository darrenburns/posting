from pathlib import Path

from textual.pilot import Pilot
from textual.widgets import Input


POSTING_MAIN = Path(__file__).parent / "posting_snapshot_app.py"


def no_cursor_blink(pilot: Pilot):
    for input in pilot.app.query(Input):
        input.cursor_blink = False


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
