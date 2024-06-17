from dataclasses import dataclass
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import VerticalScroll
from textual.screen import ModalScreen
from textual.widgets import Button, Footer, Input, Label

from posting.collection import RequestModel
from posting.save_request import FILE_SUFFIX, generate_request_filename
from posting.widgets.text_area import PostingTextArea


@dataclass
class SaveRequestData:
    """Data for the save request modal."""

    file_path: str
    """The file path of the request."""
    title: str
    """The title of the request."""
    description: str
    """The description of the request."""


class SaveRequestModal(ModalScreen[SaveRequestData | None]):
    """A modal for saving a request to disk if it has not already been saved.

    (Can also be used in situations where we're saving a copy of an existing request
    and thus want to change its name).
    """

    CSS = """
    SaveRequestModal {
        align: center middle;
    }
    """

    BINDINGS = [
        Binding("escape", "close_screen", "Cancel"),
    ]

    def __init__(self, request: RequestModel):
        super().__init__()
        self.request = request
        """The request to save."""

    def compose(self) -> ComposeResult:
        yield Footer()

    @property
    def generated_filename(self) -> str:
        request = self.request
        return generate_request_filename(request)

    def action_close_screen(self) -> None:
        self.dismiss(None)

    @on(Button.Pressed, selector="#save-button")
    def on_save(self) -> None:
        file_path = self.query_one("#save-path-input", Input).value
        generated_filename = self.generated_filename
        if not file_path:
            file_path = generated_filename + FILE_SUFFIX
        else:
            file_path += FILE_SUFFIX

        title = self.query_one("#title-input", Input).value
        if not title:
            title = generated_filename
        description = self.query_one("#description-textarea", PostingTextArea).text
        self.dismiss(
            SaveRequestData(
                file_path=file_path,
                title=title,
                description=description,
            )
        )
