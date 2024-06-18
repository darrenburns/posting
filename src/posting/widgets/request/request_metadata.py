from dataclasses import dataclass
from textual import on
from textual.app import ComposeResult
from textual.containers import VerticalScroll
from textual.message import Message
from textual.reactive import Reactive, reactive
from textual.widgets import Input, Label, TextArea
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

    @dataclass
    class Saved(Message):
        request: RequestModel
        widget: "RequestMetadata"

        @property
        def control(self) -> "RequestMetadata":
            return self.widget

    request: Reactive[RequestModel | None] = reactive(None, init=False)

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
        yield Label("Title [dim]optional[/dim]")
        yield Input(placeholder=self.generated_filename, id="name-input")
        yield Label("Description [dim]optional[/dim]")
        yield PostingTextArea(id="description-textarea")

    @property
    def generated_filename(self) -> str:
        if self.request is None:
            return ""
        return generate_request_filename(self.request)

    @property
    def request_name(self) -> str:
        if self.request is None:
            return ""
        return self.request.name or self.generated_filename

    @property
    def description(self) -> str:
        if self.request is None:
            return ""
        return self.request.description or ""

    @on(Input.Changed)
    @on(TextArea.Changed)
    def _on_changed(self, event: Input.Changed | TextArea.Changed) -> None:
        event.stop()
        if self.request is not None:
            new_request = self.request.model_copy(
                update={
                    "name": self.query_one("#name-input", Input).value
                    or self.generated_filename,
                    "description": self.query_one(
                        "#description-textarea", PostingTextArea
                    ).text,
                }
            )
            self.set_reactive(RequestMetadata.request, new_request)

    @on(Input.Submitted)
    def _on_submitted(self, event: Input.Submitted) -> None:
        event.stop()
        if self.request is not None:
            self.post_message(RequestMetadata.Saved(self.request, self))
