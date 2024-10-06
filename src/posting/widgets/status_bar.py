from rich.text import Text
from textual.app import ComposeResult
from textual.containers import Horizontal
from textual.widgets import Static

from posting.collection import RequestModel

NO_REQUEST_SELECTED = "Unsaved request"


class StatusBar(Horizontal):
    """A status bar at the bottom of the screen."""

    def compose(self) -> ComposeResult:
        yield Static(NO_REQUEST_SELECTED, id="selected-request")

    def set_request_status(self, request: RequestModel | None) -> None:
        static = self.query_one(Static)
        if request is None:
            static.update(NO_REQUEST_SELECTED)
        else:
            static.update(
                Text(
                    request.path.name,
                    overflow="ellipsis",
                    no_wrap=True,
                    end="",
                )
            )
