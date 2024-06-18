from dataclasses import dataclass
from textual.app import ComposeResult
from textual.containers import VerticalScroll
from textual.message import Message
from textual.widgets import Input, Label

from posting.collection import RequestModel
from posting.widgets.text_area import PostingTextArea


class RequestMetadata(VerticalScroll):
    DEFAULT_CSS = """
    RequestMetadata {
        padding: 1 2;
        & Input {
            width: 1fr;
            margin-bottom: 1;
        }
        & Button {
            dock: bottom;
            width: 1fr;
        }
    }
    """

    @dataclass
    class Saved(Message):
        request: RequestModel
        widget: "RequestMetadata"

        @property
        def control(self) -> "RequestMetadata":
            return self.widget

    def watch_request(self, request: RequestModel | None) -> None:
        """When the request changes, update the form."""
        if request is None:
            self.query_one("#name-input", Input).value = ""
            self.query_one("#description-textarea", PostingTextArea).text = ""
        else:
            self.query_one("#name-input", Input).value = request.name or ""
            self.query_one(
                "#description-textarea", PostingTextArea
            ).text = request.description

    def compose(self) -> ComposeResult:
        self.can_focus = False
        yield Label("Name [dim]optional[/dim]")
        yield Input(placeholder="Enter a name...", id="name-input")
        yield Label("Description [dim]optional[/dim]")
        yield PostingTextArea(id="description-textarea")

    @property
    def request_name(self) -> str:
        return self.query_one("#name-input", Input).value

    @property
    def description(self) -> str:
        return self.query_one("#description-textarea", PostingTextArea).text
