from dataclasses import dataclass
from rich.style import Style
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
            self.add_class("edit-mode")
        else:
            self.button.label = "Add"
            self.remove_class("edit-mode")

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
        Binding("escape", "cancel_edit_row", "Cancel edit"),
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

        self._row_being_edited_prior_state: tuple[str, str] | None = None
        """If the edit was cancelled, this will be the original values of the row that we revert to."""

    def on_mount(self) -> None:
        self.app.theme_changed_signal.subscribe(self, self.on_theme_change)

    def compose(self) -> ComposeResult:
        self.set_class(self.table.row_count == 0, "empty")
        yield CenterMiddle(Label(self.empty_message), id="empty-message")
        yield self.table
        yield self.key_value_input

    def on_theme_change(self, _) -> None:
        # If a row is being edited we need to refresh the cells that are being edited since they
        # will have a style that was defined by the theme at the time we entered edit mode.
        if self.row_being_edited is None:
            return

        self.highlight_and_retrieve_row_values(self.row_being_edited)

    def check_action(self, action: str, parameters: tuple[object, ...]) -> bool | None:
        if action == "cancel_edit_row":
            return self.row_being_edited is not None
        return super().check_action(action, parameters)

    @on(KeyValueInput.Change)
    def add_key_value_pair(self, event: KeyValueInput.Change) -> None:
        if self.row_being_edited is None:
            # Adding a new row.
            table = self.table
            table.add_row(event.key, event.value, sender=table)
            table.move_cursor(row=table.row_count - 1)
        else:
            # Editing an existing row.
            table = self.table
            row_key = self.row_being_edited
            row = table._row_locations.get(row_key)
            if row is None:
                log.warning(f"Row {row_key} not found")
                return

            table.update_cell_at(Coordinate(row, 0), event.key)
            table.update_cell_at(Coordinate(row, 1), event.value)
            self.row_being_edited = None
            self.table.column_width_refresh()
            # Move the focus back from the input to the table.
            self.table.focus()

    @on(PostingDataTable.RowsRemoved)
    def rows_removed(self, event: PostingDataTable.RowsRemoved) -> None:
        rows = event.data_table.row_count

        print(f"rows_removed: {rows}")
        if self.row_being_edited is not None:
            print(f"row_being_edited: {self.row_being_edited}")
            self.action_cancel_edit_row()

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

        if self.row_being_edited is not None:
            self.action_cancel_edit_row()

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
            # No longer editing a row.
            # This stuff is handled in the action_cancel_edit_row method.
            return

        # A row is now being edited.
        self.key_value_input.edit_mode = True

        # Grab the values from the row
        key, val = self.highlight_and_retrieve_row_values(row_key)

        self._row_being_edited_prior_state = (key, val)
        self.key_value_input.key_input.value = key
        self.key_value_input.value_input.value = val
        self.key_value_input.key_input.focus()

    def highlight_and_retrieve_row_values(self, row_key: RowKey) -> tuple[str, str]:
        row_values = self.table.get_row(row_key)
        key = row_values[0].plain if isinstance(row_values[0], Text) else row_values[0]
        val = row_values[1].plain if isinstance(row_values[1], Text) else row_values[1]

        # Highlight the text of the row, to indicate that it is being edited.
        row_index = self.table._row_locations.get(row_key)
        accent_color = self.app.theme_variables.get("text-warning")
        self.table.update_cell_at(
            Coordinate(row_index, 0),
            Text(key, style=Style(color=accent_color, italic=True)),
        )
        self.table.update_cell_at(
            Coordinate(row_index, 1),
            Text(val, style=Style(color=accent_color, italic=True)),
        )
        return key, val

    def action_cancel_edit_row(self) -> None:
        if self.row_being_edited is None or self._row_being_edited_prior_state is None:
            return

        # Revert the row to its original state.
        old_key, old_val = self._row_being_edited_prior_state
        row_index = self.table._row_locations.get(self.row_being_edited)

        self.row_being_edited = None
        self.key_value_input.edit_mode = False
        self.key_value_input.key_input.value = ""
        self.key_value_input.value_input.value = ""

        # The row index could be None if the row was deleted, as in that
        # case the lookup above will return None.
        if row_index is not None:
            self.table.update_cell_at(Coordinate(row_index, 0), old_key)
            self.table.update_cell_at(Coordinate(row_index, 1), old_val)
            self.table.focus()
