import asyncio

from textual.app import App, ComposeResult

from posting.config import SETTINGS, Settings
from posting.widgets.text_area import ReadOnlyTextArea
from test_snapshots import use_config


class ReadOnlyTextAreaApp(App[None]):
    def __init__(self) -> None:
        SETTINGS.set(Settings())
        super().__init__()

    def compose(self) -> ComposeResult:
        yield ReadOnlyTextArea("\n".join(f"line {i}" for i in range(100)))


def run_keys(*keys: str, start: tuple[int, int] = (0, 0)) -> ReadOnlyTextArea:
    async def run() -> ReadOnlyTextArea:
        app = ReadOnlyTextAreaApp()
        async with app.run_test(size=(40, 20)) as pilot:
            text_area = app.query_one(ReadOnlyTextArea)
            text_area.focus()
            text_area.move_cursor(start)
            await pilot.press(*keys)
            return text_area

    return asyncio.run(run())


@use_config("general.yaml")
def test_half_page_down_extends_selection_in_visual_mode():
    text_area = run_keys("v", "ctrl+d")

    assert text_area.selection.start == (0, 0)
    assert text_area.selection.end[0] > 0


@use_config("general.yaml")
def test_half_page_up_extends_selection_in_visual_mode():
    text_area = run_keys("v", "ctrl+u", start=(50, 0))

    assert text_area.selection.start == (50, 0)
    assert text_area.selection.end[0] < 50


@use_config("general.yaml")
def test_half_page_down_moves_cursor_without_visual_mode():
    text_area = run_keys("ctrl+d")

    assert text_area.selection.is_empty
    assert text_area.selection.end[0] > 0
