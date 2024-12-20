from textual.app import ComposeResult
from textual.containers import Vertical
from textual.widgets import Label


class Replies(Vertical):
    def compose(self) -> ComposeResult:
        self.border_title = "Incoming"
        yield Label("Replies")
