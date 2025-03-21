from dataclasses import dataclass
from rich.text import Text
from textual import on, log
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.coordinate import Coordinate
from textual.message import Message
from textual.reactive import Reactive, reactive
from textual.widget import Widget
from textual.widgets import Button, Input, Label
from textual.widgets.data_table import RowKey
from posting.widgets.center_middle import CenterMiddle

from posting.widgets.datatable import PostingDataTable


class KeyValueInput(Horizontal):
    @dataclass
    class Change(Message):
        key: str
        value: str
        _control: "KeyValueInput"

        @property
        def control(self) -> "KeyValueInput":
            return self._control

    edit_mode: Reactive[bool] = reactive(False)

    def __init__(
        self,
        key_input: Input,
        value_input: Input,
        value_required: bool = False,
        button_label: str = "Add",
        *children: Widget,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(
            *children, name=name, id=id, classes=classes, disabled=disabled
        )
        self.key_input = key_input
        self.value_input = value_input
        self.value_required = value_required
        self.button_label = button_label

    def compose(self) -> ComposeResult:
        self.key_input.add_class("key-input")
        self.value_input.add_class("value-input")
        yield self.key_input
        yield self.value_input
        add_button = Button(self.button_label, disabled=True, id="add-button")
        add_button.can_focus = False
        yield add_button

    def watch_edit_mode(self, edit_mode: bool) -> None:
        if edit_mode:
            self.button.label = "Update"
        else:
            self.button.label = "Add"

    @property
    def submit_allowed(self) -> bool:
        has_key = bool(self.key_input.value)
        has_value = bool(self.value_input.value)
        return has_key and (not self.value_required or has_value)

    @on(Input.Changed)
    def determine_button_enabled(self) -> None:
        button = self.query_one("#add-button", Button)
        button.disabled = not self.submit_allowed

    @on(Input.Submitted)
    @on(Button.Pressed)
    def add_pair(self, event: Input.Submitted | Button.Pressed) -> None:
        key_input = self.key_input
        value_input = self.value_input
        key = key_input.value
        value = value_input.value

        def add() -> None:
            self.post_message(
                self.Change(
                    key=key,
                    value=value,
                    _control=self,
                )
            )
            key_input.clear()
            value_input.clear()
            key_input.focus()

        if key and value:
            add()
        elif key and not value:
            if isinstance(event, Input.Submitted):
                input_widget = event.input
                if input_widget.has_class("key-input"):
                    value_input.focus()
                elif input_widget.has_class("value-input") and self.submit_allowed:
                    add()
            else:
                add()
        elif value and not key:
            key_input.focus()
        else:
            # Case where both are empty - do nothing.
            pass

    @property
    def button(self) -> Button:
        return self.query_one("#add-button", Button)


class KeyValueEditor(Vertical):
    BINDINGS = [
        Binding("enter", "edit_row", "Edit row"),
        # TODO - implement check_action
        Binding("escape", "cancel_edit_row", "Cancel edit row"),
    ]

    row_being_edited: Reactive[RowKey | None] = reactive(None)
    """The row that is currently being edited, or None if no row is being edited."""

    def __init__(
        self,
        table: PostingDataTable,
        key_value_input: KeyValueInput,
        empty_message: str = "No entries",
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(name=name, id=id, classes=classes, disabled=disabled)
        self.table = table
        self.key_value_input = key_value_input
        self.empty_message = empty_message

    def compose(self) -> ComposeResult:
        self.set_class(self.table.row_count == 0, "empty")
        yield CenterMiddle(Label(self.empty_message), id="empty-message")
        yield self.table
        yield self.key_value_input

    @on(KeyValueInput.Change)
    def add_key_value_pair(self, event: KeyValueInput.Change) -> None:
        if self.row_being_edited is None:
            table = self.table
            table.add_row(event.key, event.value, sender=table)
            table.move_cursor(row=table.row_count - 1)
        else:
            table = self.table
            row_key = self.row_being_edited
            row = table._row_locations.get(row_key)
            if row is None:
                log.warning(f"Row {row_key} not found")
                return
            table.update_cell_at(Coordinate(row, 0), event.key)
            table.update_cell_at(Coordinate(row, 1), event.value)
            self.row_being_edited = None

    @on(PostingDataTable.RowsRemoved)
    def rows_removed(self, event: PostingDataTable.RowsRemoved) -> None:
        rows = event.data_table.row_count
        self.set_class(rows == 0, "empty")
        if rows == 0 and event.explicit_by_user:
            self.key_value_input.key_input.focus()

    @on(PostingDataTable.RowsAdded)
    def rows_added(self, event: PostingDataTable.RowsAdded) -> None:
        rows = event.data_table.row_count
        self.set_class(rows == 0, "empty")

    @on(PostingDataTable.RowSelected)
    def row_selected(self, event: PostingDataTable.RowSelected) -> None:
        """Switch to edit mode when a row is selected."""
        # Update the row that is currently being edited.
        table = self.table
        cursor_row_index = table.cursor_row
        row_key, _col_key = table.coordinate_to_cell_key(
            Coordinate(cursor_row_index, 0)
        )
        self.row_being_edited = row_key

    def watch_row_being_edited(self, row_key: RowKey | None) -> None:
        """Handle edit mode enable/disable."""
        if row_key is None:
            self.key_value_input.edit_mode = False
            self.key_value_input.remove_class("edit-mode")
            return

        self.key_value_input.add_class("edit-mode")

        # Get the values from the row, and store them so that if we cancel the edit,
        # we can revert the row to its original state.
        row_values = self.table.get_row(row_key)
        self.key_value_input.edit_mode = True
        self.key_value_input.key_input.value = (
            row_values[0].plain if isinstance(row_values[0], Text) else row_values[0]
        )
        self.key_value_input.value_input.value = (
            row_values[1].plain if isinstance(row_values[1], Text) else row_values[1]
        )
        self.key_value_input.key_input.focus()

    def action_cancel_edit_row(self) -> None:
        if self.row_being_edited is None:
            return

        self.key_value_input.edit_mode = False
        self.row_being_edited = None
        self.key_value_input.key_input.value = ""
        self.key_value_input.value_input.value = ""
