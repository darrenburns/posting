from textual.app import ComposeResult
from textual.containers import VerticalScroll
from textual.reactive import Reactive, reactive
from textual.widgets import Button, Input, Label
from posting.collection import RequestModel
from posting.save_request import generate_request_filename

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

    request: Reactive[RequestModel | None] = reactive(None)

    def compose(self) -> ComposeResult:
        self.can_focus = False
        yield Label("Save path [dim]optional[/dim]")
        yield Input(placeholder=self.generated_filename, id="save-path-input")
        yield Label("Title [dim]optional[/dim]")
        yield Input(placeholder=self.generated_filename, id="title-input")
        yield Label("Description [dim]optional[/dim]")
        yield PostingTextArea(id="description-textarea")
        yield Button.success("Save", id="save-button")

    @property
    def generated_filename(self) -> str:
        if self.request is None:
            return ""
        return generate_request_filename(self.request)
