from textual.app import ComposeResult
from textual.containers import VerticalScroll
from textual.reactive import Reactive, reactive
from textual.widgets import Input, Label

from posting.collection import RequestModel
from posting.widgets.text_area import PostingTextArea, ReadOnlyTextArea
from posting.widgets.variable_input import VariableInput


class RequestMetadata(VerticalScroll):
    request: Reactive[RequestModel | None] = reactive(None, init=False)

    def watch_request(self, request: RequestModel | None) -> None:
        """When the request changes, update the form."""
        if request is None:
            self.request_name_input.value = ""
            self.request_description_textarea.text = ""
            self.request_path_text_area.text = "Request not saved to disk."
        else:
            self.request_name_input.value = request.name or ""
            self.request_description_textarea.text = request.description
            self.request_path_text_area.text = (
                str(request.path) if request.path else "Request not saved to disk."
            )

    def compose(self) -> ComposeResult:
        self.can_focus = False
        yield Label("Name [dim]optional[/dim]")
        yield VariableInput(placeholder="Enter a name…", id="name-input")
        yield Label("Description [dim]optional[/dim]")
        yield PostingTextArea(id="description-textarea")
        yield Label("Path [dim]read-only[/dim]")
        yield ReadOnlyTextArea(
            "Request not saved to disk.", select_on_focus=True, id="request-path"
        )

    @property
    def request_name_input(self) -> Input:
        return self.query_one("#name-input", Input)

    @property
    def request_description_textarea(self) -> PostingTextArea:
        return self.query_one("#description-textarea", PostingTextArea)

    @property
    def request_path_text_area(self) -> ReadOnlyTextArea:
        return self.query_one("#request-path", ReadOnlyTextArea)

    @property
    def request_name(self) -> str:
        return self.request_name_input.value

    @property
    def description(self) -> str:
        return self.request_description_textarea.text
