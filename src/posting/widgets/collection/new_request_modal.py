from dataclasses import dataclass
from typing import TYPE_CHECKING
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, VerticalScroll
from textual.screen import ModalScreen
from textual.validation import Length, ValidationResult, Validator
from textual.widgets import Button, Footer, Input, Label
from textual.widgets.tree import TreeNode
from posting.files import is_valid_filename, request_file_exists

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


class FileNameValidator(Validator):
    def validate(self, value: str) -> ValidationResult:
        return (
            self.success()
            if is_valid_filename(value)
            else self.failure("Invalid file name")
        )


class DirectoryValidator(Validator):
    def validate(self, value: str) -> ValidationResult:
        def is_valid_directory(value: str) -> bool:
            if not value or ".." in value or value.startswith("/") or ":" in value:
                return False
            return True

        return (
            self.success()
            if is_valid_directory(value)
            else self.failure("Invalid directory")
        )


class NewRequestModal(ModalScreen[NewRequestData | None]):
    """A modal for saving a request to disk if it has not already been saved.

    (Can also be used in situations where we're saving a copy of an existing request
    and thus want to change its name).
    """

    CSS = """
    NewRequestModal {
        align: center middle;
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

        & .error-label {
            color: $text-error;
            dock: right;
        }

        & .form-label-row {
            height: 1;
        }

        & #title-error-label,
        & #file-name-error-label,
        & #directory-error-label {
            display: none;
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
        with VerticalScroll(classes="modal-body") as vs:
            vs.can_focus = False
            vs.border_title = "New request"

            with Horizontal(classes="form-label-row"):
                yield Label("Title")
                yield Label(
                    "Must not be empty", id="title-error-label", classes="error-label"
                )

            yield PostingInput(
                self._initial_title,
                placeholder="Enter a title",
                validators=[
                    Length(minimum=1, failure_description="Title cannot be empty")
                ],
                id="title-input",
            )

            with Horizontal(classes="form-label-row"):
                yield Label("File name [dim]optional[/]")
                yield Label(
                    "Not valid", id="file-name-error-label", classes="error-label"
                )

            with Horizontal():
                filename_input = PostingInput(
                    placeholder="Enter a file name",
                    id="file-name-input",
                    validators=[FileNameValidator()],
                )
                # Empty is valid, because we'll generate a filename if the user leaves
                # the file name input blank.
                filename_input.valid_empty = True
                yield filename_input
                yield Label(".posting.yaml", id="file-suffix-label")

            yield Label("Description [dim]optional[/]")
            yield PostingTextArea(
                self._initial_description,
                id="description-textarea",
                show_line_numbers=False,
            )

            with Horizontal(classes="form-label-row"):
                yield Label("Path in collection")
                yield Label(
                    "Not valid", id="directory-error-label", classes="error-label"
                )

            yield PostingInput(
                self._initial_directory,
                placeholder="Enter a path to save the request to",
                id="directory-input",
                validators=[DirectoryValidator()],
            )

            yield Button.success("Create request", id="create-button")

        yield Footer(show_command_palette=False)

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
        file_name_input = self.file_name_input
        file_name_input.value = self._generated_filename
        file_name_input.placeholder = self._generated_filename
        file_name_input.cursor_position = len(self._generated_filename)
        file_name_input.refresh()

    @on(Input.Submitted)
    @on(Button.Pressed, selector="#create-button")
    def on_create(self, event: Input.Submitted | Button.Pressed) -> None:
        self.validate_and_create_request()

    def action_create_request(self) -> None:
        self.validate_and_create_request()

    def validate_and_create_request(
        self,
    ) -> None:
        title_input = self.title_input
        file_name_input = self.file_name_input
        description_textarea = self.description_textarea
        directory_input = self.directory_input

        directory_validation_result = directory_input.validate(directory_input.value)
        file_name_validation_result = file_name_input.validate(file_name_input.value)
        title_validation_result = title_input.validate(title_input.value)

        title_error_label = self.query_one("#title-error-label", Label)
        if title_validation_result is not None and not title_validation_result.is_valid:
            description = title_validation_result.failures[0].description
            if description is not None:
                title_error_label.update(description)
            title_error_label.display = "block"
            return
        else:
            title_error_label.display = "none"

        file_name_error_label = self.query_one("#file-name-error-label", Label)
        if (
            file_name_validation_result is not None
            and not file_name_validation_result.is_valid
        ):
            description = file_name_validation_result.failures[0].description
            if description is not None:
                file_name_error_label.update(description)
            file_name_error_label.display = "block"
            return
        else:
            file_name_error_label.display = "none"

        directory_error_label = self.query_one("#directory-error-label", Label)
        if (
            directory_validation_result is not None
            and not directory_validation_result.is_valid
        ):
            description = directory_validation_result.failures[0].description
            if description is not None:
                directory_error_label.update(description)
            directory_error_label.display = "block"
            return
        else:
            directory_error_label.display = "none"

        file_name = file_name_input.value
        title = title_input.value
        description = description_textarea.text
        directory = directory_input.value

        generated_filename = self._generated_filename
        if not file_name:
            file_name = generated_filename + FILE_SUFFIX
        else:
            file_name += FILE_SUFFIX

        if not title:
            title = generated_filename

        # Check if a request with the same name already exists in the collection.
        if self._parent_node is not None:
            parent_path = (
                self._parent_node.data.path if self._parent_node.data else None
            )
            if parent_path is not None:
                if request_file_exists(file_name, parent_path):
                    self.notify(
                        "A request with this name already exists.",
                        severity="error",
                    )
                    return

        self.dismiss(
            NewRequestData(
                file_name=file_name,
                title=title,
                description=description,
                directory=directory,
            )
        )

    @property
    def file_name_input(self) -> Input:
        return self.query_one("#file-name-input", Input)

    @property
    def title_input(self) -> Input:
        return self.query_one("#title-input", Input)

    @property
    def description_textarea(self) -> PostingTextArea:
        return self.query_one("#description-textarea", PostingTextArea)

    @property
    def directory_input(self) -> Input:
        return self.query_one("#directory-input", Input)
