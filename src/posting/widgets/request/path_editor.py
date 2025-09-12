from rich.text import Text
from textual.app import ComposeResult
from textual.containers import Vertical
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

    def __init__(self) -> None:
        super().__init__(
            PathParamsTable(),
            KeyValueInput(
                Input(placeholder="Key", id="path-key-input", disabled=True),
                VariableInput(placeholder="Value"),
                button_label="Update",
            ),
            empty_message="No path parameters in URL",
        )

    def add_key_value_pair(self, event: KeyValueInput.Change) -> None:  # type: ignore[override]
        # Only allow updates to existing rows. Do nothing if no row is selected for editing.
        if self._row_being_edited is None:
            return
        return super().add_key_value_pair(event)

    def enter_edit_mode(self, row_key: RowKey, focus_value: bool = False) -> None:
        super().enter_edit_mode(row_key, focus_value=True)
        # Ensure the key field stays disabled and we focus value.
        self.key_value_input.key_input.disabled = True
        self.key_value_input.value_input.focus()


class PathEditor(Vertical):
    """
    The Path tab which contains the path parameter editor.
    """

    def compose(self) -> ComposeResult:
        yield PathParamsEditor()

    @property
    def path_key_input(self) -> Input:
        return self.query_one("#path-key-input", Input)
