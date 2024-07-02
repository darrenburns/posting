from dataclasses import dataclass
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.message import Message
from textual.widget import Widget
from textual.widgets import Button, Input, Label
from posting.widgets.center_middle import CenterMiddle

from posting.widgets.datatable import PostingDataTable


class KeyValueInput(Horizontal):
    DEFAULT_CSS = """\
    KeyValueInput {
        height: auto;
        width: 1fr;
        & > Input {
            border: none;
            width: 1fr;
            margin-left: 1;
            &:focus {
                border: none;
                padding: 0 1;
            }
        }

        & > Button {
            background: $primary;
            color: $text;
            text-style: none;
            width: 10;
            margin: 0 1;
            &:hover {
                text-style: b;
                padding: 0 1;
                border: none;
                background: $primary-darken-1;
            }
        }
    }
    """

    BINDINGS = [
        # TODO - textual-autocomplete is eating the up key.
        Binding("up", "screen.focus_previous", "Focus previous"),
    ]

    @dataclass
    class New(Message):
        key: str
        value: str
        _control: "KeyValueInput"

        @property
        def control(self) -> "KeyValueInput":
            return self._control

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
            self.post_message(self.New(key=key, value=value, _control=self))
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


class KeyValueEditor(Vertical):
    DEFAULT_CSS = """\
    KeyValueEditor {
        & KeyValueInput {
            dock: bottom;
        }
        & PostingDataTable {
            display: block;
        }

        & #empty-message {
            display: none;
        }

        &.empty {
            & PostingDataTable {
                display: none;
            }
            & #empty-message {
                hatch: right $surface-lighten-1 70%;
                display: block;
            }
        }
    }
    """

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

    @on(KeyValueInput.New)
    def add_key_value_pair(self, event: KeyValueInput.New) -> None:
        table = self.table
        table.add_row(event.key, event.value)
        table.move_cursor(row=table.row_count - 1)

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
