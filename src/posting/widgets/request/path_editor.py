from dataclasses import dataclass
from textual.binding import Binding
from rich.text import Text
from textual.app import ComposeResult
from textual.containers import Vertical
from textual.message import Message
from textual.widgets import Input
from textual.widgets.data_table import RowKey

from posting.collection import PathParam
from posting.widgets.datatable import PostingDataTable
from posting.widgets.key_value import KeyValueEditor, KeyValueInput
from posting.widgets.variable_input import VariableInput


class PathParamsTable(PostingDataTable):
    """
    Table of path parameters extracted from the URL.

    Rows are controlled by the URL. Users cannot add or remove rows manually.
    """

    def on_mount(self):
        self.fixed_columns = 0
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.row_disable = False
        self.add_columns("Key", "Value")

    def action_remove_row(self) -> None:
        # Disallow manual row removal.
        return

    def to_model(self) -> list[PathParam]:
        params: list[PathParam] = []
        for row_index in range(self.row_count):
            row = self.get_row_at(row_index)
            params.append(
                PathParam(
                    name=row[0].plain if isinstance(row[0], Text) else row[0],
                    value=row[1].plain if isinstance(row[1], Text) else row[1],
                )
            )
        return params


class PathParamsEditor(KeyValueEditor):
    """
    Editor for path parameters. Users may only edit values, not add or remove rows.
    """

    BINDINGS = [
        Binding(
            "alt+down", "jump_to_url_param", "Jump to param in URL bar", show=False
        ),
    ]

    @dataclass
    class PathParamsUpdated(Message):
        params: dict[str, str]

    @dataclass
    class PathParamJumpRequestedFromPathEditor(Message):  # type: ignore[misc]
        name: str
        editor: "PathParamsEditor"

        @property
        def control(self) -> "PathParamsEditor":
            return self.editor

    def __init__(self) -> None:
        super().__init__(
            PathParamsTable(),
            KeyValueInput(
                Input(placeholder="Key", id="path-key-input", disabled=True),
                VariableInput(placeholder="Value"),
                button_label="Update",
            ),
            empty_message=(
                "[b]No path parameters in URL[/]\n"
                "Use [$accent]:param[/] syntax add path parameters.\n"
                "e.g. http://example.com/:param1/:param2"
            ),
        )
        # Disable value input until a row is selected for editing.
        self.key_value_input.value_input.disabled = True

    def on_mount(self) -> None:
        # Hide the action button unless we're editing a row.
        self.key_value_input.button.display = False

    def add_key_value_pair(self, event: KeyValueInput.Change) -> None:
        # Only allow updates to existing rows. Do nothing if no row is selected for editing.
        if self._row_being_edited is None:
            return
        super().add_key_value_pair(event)
        params = self._get_params()
        self.post_message(self.PathParamsUpdated(params))

    def enter_edit_mode(self, row_key: RowKey, focus_value: bool = False) -> None:
        super().enter_edit_mode(row_key, focus_value=True)
        # Ensure the key field stays disabled and we focus value.
        self.key_value_input.key_input.disabled = True
        self.key_value_input.value_input.disabled = False
        self.key_value_input.value_input.focus()
        # Show the action button while editing.
        self.key_value_input.button.display = True

    def exit_edit_mode(self, revert: bool = False) -> None:
        if self._row_being_edited is None:
            return
        super().exit_edit_mode(revert)
        # After exiting edit mode, prevent focusing value input.
        self.key_value_input.value_input.disabled = True
        # Hide the action button when not editing.
        self.key_value_input.button.display = False
        params = self._get_params()
        self.post_message(self.PathParamsUpdated(params))

    def action_jump_to_url_param(self) -> None:
        """Post a message requesting a jump to the corresponding param in the URL bar."""
        table = self.table
        row_index = table.cursor_row
        if row_index < 0 or row_index >= table.row_count:
            return
        row = table.get_row_at(row_index)
        key_cell = row[0]
        name = key_cell.plain if isinstance(key_cell, Text) else key_cell
        self.exit_edit_mode(revert=True)
        self.post_message(
            self.PathParamJumpRequestedFromPathEditor(name=str(name), editor=self)
        )

    def _get_params(self) -> dict[str, str]:
        params: dict[str, str] = {}
        for row_index in range(self.table.row_count):
            row = self.table.get_row_at(row_index)
            key = row[0].plain if isinstance(row[0], Text) else row[0]
            val = row[1].plain if isinstance(row[1], Text) else row[1]
            params[str(key)] = str(val)
        return params


class PathEditor(Vertical):
    """
    The Path tab which contains the path parameter editor.
    """

    def compose(self) -> ComposeResult:
        yield PathParamsEditor()

    @property
    def path_key_input(self) -> Input:
        return self.query_one("#path-key-input", Input)
