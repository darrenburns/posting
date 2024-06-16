from dataclasses import dataclass
from pathlib import Path
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import VerticalScroll
from textual.screen import ModalScreen
from textual.widgets import Button, Footer, Input, Label, TextArea

from posting.collection import RequestModel
from posting.widgets.text_area import PostingTextArea


@dataclass
class SaveRequestData:
    """Data for the save request modal."""

    path: Path
    """The path to save the request to."""


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
            vs.border_title = "Save new request"
            yield Label("Title")
            yield Input()
            yield Label("Description")
            yield PostingTextArea()
            yield Button.success("Save")
        yield Footer()
