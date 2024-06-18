from dataclasses import dataclass
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, VerticalScroll
from textual.screen import ModalScreen
from textual.widgets import Button, Footer, Input, Label

from posting.save_request import FILE_SUFFIX, generate_request_filename
from posting.widgets.text_area import PostingTextArea


@dataclass
class NewRequestData:
    """Data for the save request modal."""

    title: str
    """The title of the request."""
    file_name: str
    """The file name of the request."""
    description: str
    """The description of the request."""


class NewRequestModal(ModalScreen[NewRequestData | None]):
    """A modal for saving a request to disk if it has not already been saved.

    (Can also be used in situations where we're saving a copy of an existing request
    and thus want to change its name).
    """

    CSS = """
    NewRequestModal {
        align: center middle;
        & > VerticalScroll {
            background: $background;
            padding: 1 2;
            width: 50%;
            height: 70%;
            border: wide $background-lighten-2;
            border-title-color: $text;
            border-title-background: $background;
            border-title-style: bold;
        }

        & Input, & PostingTextArea {
            margin-bottom: 1;
            width: 1fr;
        }

        & Horizontal {
            height: 2;
        }

        & Button {
            width: 1fr;
        }

        & #file-suffix-label {
            width: auto;
            height: 1;
            background: $surface;
            color: $text-muted;
        }
    }
    """

    BINDINGS = [
        Binding("escape", "close_screen", "Cancel"),
    ]

    def compose(self) -> ComposeResult:
        with VerticalScroll() as vs:
            vs.can_focus = False
            vs.border_title = "New request"

            yield Label("Title")
            yield Input(placeholder="Enter a title", id="title-input")

            yield Label("Description [dim]optional[/dim]")
            yield PostingTextArea(id="description-textarea")

            yield Label("File name [dim]optional[/dim]")
            with Horizontal():
                yield Input(placeholder="Enter a file name", id="save-path-input")
                yield Label(".posting.yaml", id="file-suffix-label")

            yield Button.success("Create request", id="create-button")

        yield Footer()

    def action_close_screen(self) -> None:
        self.dismiss(None)

    @on(Input.Changed, selector="#title-input")
    def on_title_changed(self, event: Input.Changed) -> None:
        """When the title changes, update the generated filename.

        This is the filename that will be used if the user leaves the file name
        input blank.
        """
        value = event.value
        self._generated_filename = generate_request_filename(value)
        self.query_one("#save-path-input", Input).placeholder = self._generated_filename

    @on(Input.Submitted)
    @on(Button.Pressed, selector="#create-button")
    def on_create(self, event: Input.Submitted | Button.Pressed) -> None:
        file_name = self.query_one("#save-path-input", Input).value
        generated_filename = self._generated_filename
        if not file_name:
            file_name = generated_filename + FILE_SUFFIX
        else:
            file_name += FILE_SUFFIX

        title = self.query_one("#title-input", Input).value
        if not title:
            title = generated_filename

        description = self.query_one("#description-textarea", PostingTextArea).text

        self.dismiss(
            NewRequestData(
                file_name=file_name,
                title=title,
                description=description,
            )
        )
