from textual.app import ComposeResult
from textual.containers import Vertical
from textual.widgets import Label


class SnippetsLibrary(Vertical):
    def compose(self) -> ComposeResult:
        yield Label("Snippets")
