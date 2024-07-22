from textual.widgets import Input

from posting.config import SETTINGS


class PostingInput(Input):
    def on_mount(self) -> None:
        self.cursor_blink = SETTINGS.get().text_input.blinking_cursor
