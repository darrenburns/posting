from textual.app import ComposeResult
from textual.containers import VerticalScroll


class RequestAuth(VerticalScroll):
    def compose(self) -> ComposeResult:
        return super().compose()
