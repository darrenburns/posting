from textual.app import ComposeResult
from textual.containers import Horizontal
from textual.widgets import Label

from posting.collection import RequestModel

NO_REQUEST_SELECTED = "Unsaved request"


class StatusBar(Horizontal):
    """A status bar at the bottom of the screen."""

    def compose(self) -> ComposeResult:
        yield Label(NO_REQUEST_SELECTED, id="selected-request")

    def set_request_status(self, request: RequestModel | None) -> None:
        label = self.query_one(Label)
        if request is None:
            label.update(NO_REQUEST_SELECTED)
        else:
            label.update(f"Editing [b]{request.name}[/]")
