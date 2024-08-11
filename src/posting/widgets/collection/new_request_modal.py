from dataclasses import dataclass
from typing import TYPE_CHECKING
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, VerticalScroll
from textual.screen import ModalScreen
from textual.validation import ValidationResult, Validator
from textual.widgets import Button, Footer, Input, Label
from textual.widgets.tree import TreeNode

from posting.save_request import FILE_SUFFIX, generate_request_filename
from posting.widgets.input import PostingInput
from posting.widgets.text_area import PostingTextArea

if TYPE_CHECKING:
    from posting.widgets.collection.browser import CollectionNode


@dataclass
class NewRequestData:
    """Data for the save request modal."""

    title: str
    """The title of the request."""
    file_name: str
    """The file name of the request."""
    description: str
    """The description of the request."""
    directory: str
    """The directory of the request."""


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

    AUTO_FOCUS = "#title-input"
    BINDINGS = [
        Binding("escape", "close_screen", "Cancel"),
        Binding("ctrl+n,alt+enter", "create_request", "Create"),
    ]

    def __init__(
        self,
        initial_directory: str,
        initial_title: str = "",
        initial_description: str = "",
        parent_node: TreeNode["CollectionNode"] | None = None,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
    ) -> None:
        super().__init__(name, id, classes)
        self._initial_title = initial_title
        self._initial_description = initial_description
        self._initial_directory = initial_directory
        self._generated_filename = ""
        self._parent_node = parent_node

    def compose(self) -> ComposeResult:
        with VerticalScroll() as vs:
            vs.can_focus = False
            vs.border_title = "New request"

            yield Label("Title")
            yield PostingInput(
                self._initial_title,
                placeholder="Enter a title",
                id="title-input",
            )

            yield Label("File name [dim]optional[/dim]")
            with Horizontal():
                yield PostingInput(
                    placeholder="Enter a file name", id="file-name-input"
                )
                yield Label(".posting.yaml", id="file-suffix-label")

            yield Label("Description [dim]optional[/dim]")
            yield PostingTextArea(
                self._initial_description,
                id="description-textarea",
                show_line_numbers=False,
            )

            yield Label("Directory")
            yield PostingInput(
                self._initial_directory,
                placeholder="Enter a directory",
                id="directory-input",
            )

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
        file_name_input = self.query_one("#file-name-input", Input)
        file_name_input.placeholder = self._generated_filename
        file_name_input.refresh()

    @on(Input.Submitted)
    @on(Button.Pressed, selector="#create-button")
    def on_create(self, event: Input.Submitted | Button.Pressed) -> None:
        self.create_request()

    def action_create_request(self) -> None:
        self.create_request()

    def create_request(self) -> None:
        file_name_input = self.query_one("#file-name-input", Input)
        file_name = file_name_input.value
        directory = self.query_one("#directory-input", Input).value
        description = self.query_one("#description-textarea", PostingTextArea).text

        generated_filename = self._generated_filename
        if not file_name:
            file_name = generated_filename + FILE_SUFFIX
        else:
            file_name += FILE_SUFFIX

        title = self.query_one("#title-input", Input).value
        if not title:
            title = generated_filename

        # Check if a request with the same name already exists in the collection.
        if self._parent_node is not None:
            parent_path = (
                self._parent_node.data.path if self._parent_node.data else None
            )
            if parent_path is not None:
                for path in parent_path.iterdir():
                    stem = path.stem
                    i = stem.rfind(".")
                    if 0 < i < len(stem) - 1:
                        name = stem[:i]
                        new_file_stem = file_name[: len(file_name) - len(FILE_SUFFIX)]
                        print(name, new_file_stem)
                        if name == new_file_stem:
                            # Don't create duplicates. Notify and return.
                            self.notify(
                                "A request with this name already exists.",
                                severity="error",
                            )
                            file_name_input.focus()
                            return
                    else:
                        continue

        self.dismiss(
            NewRequestData(
                file_name=file_name,
                title=title,
                description=description,
                directory=directory,
            )
        )
