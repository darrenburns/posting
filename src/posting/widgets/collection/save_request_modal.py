from dataclasses import dataclass
from pathlib import Path
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import VerticalScroll
from textual.screen import ModalScreen
from textual.widgets import Button, Footer, Input, Label, TextArea

from posting.collection import RequestModel
from posting.save_request import generate_request_filename
from posting.widgets.text_area import PostingTextArea


@dataclass
class SaveRequestData:
    """Data for the save request modal."""

    file_name: str
    """The file name of the request."""
    title: str
    """The title of the request."""
    description: str
    """The description of the request."""


class SaveRequestModal(ModalScreen[SaveRequestData]):
    """A modal for saving a request to disk if it has not already been saved.

    (Can also be used in situations where we're saving a copy of an existing request
    and thus want to change its name).
    """

    CSS = """
    SaveRequestModal {

        & VerticalScroll {
            width: 50%;
            height: auto;
            max-height: 70%;
            & Input {
                width: 100%;
                margin-bottom: 1;
            }
            & TextArea {
                width: 100%;
                height: 4;
                padding: 0;
                background: $surface;
                margin-bottom: 1;
                &:focus {
                    background: $surface-lighten-1;
                    border-left: outer $surface-lighten-2;
                }
            }
            & Button {
                dock: bottom;
                width: 1fr;
            }
        }


    }
    """

    BINDINGS = [
        Binding("escape", "app.pop_screen", "Cancel"),
    ]

    def __init__(self, request: RequestModel):
        super().__init__()
        self.request = request
        """The request to save."""

    def compose(self) -> ComposeResult:
        with VerticalScroll() as vs:
            vs.can_focus = False
            vs.border_title = "Save new request"
            yield Label("File name [dim]optional[/dim]")
            yield Input(placeholder=self.generated_filename, id="file-name-input")
            yield Label("Title [dim]optional[/dim]")
            yield Input(id="title-input")
            yield Label("Description [dim]optional[/dim]")
            yield PostingTextArea(id="description-textarea")
            yield Button.success("Save", id="save-button")
        yield Footer()

    @property
    def generated_filename(self) -> str:
        request = self.request
        return generate_request_filename(request.method, request.name)

    @on(Button.Pressed, selector="#save-button")
    def on_save(self) -> None:
        file_name = self.query_one("#file-name-input", Input).value
        if not file_name:
            file_name = self.generated_filename
        title = self.query_one("#title-input", Input).value
        description = self.query_one("#description-textarea", PostingTextArea).text
        self.dismiss(
            SaveRequestData(
                file_name=file_name,
                title=title,
                description=description,
            )
        )
