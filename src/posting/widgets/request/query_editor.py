from dataclasses import dataclass

from rich.text import Text
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical, Horizontal
from textual.message import Message
from textual.widgets import DataTable, Input, Button
from textual.widgets.data_table import RowKey, CellDoesNotExist

from posting.widgets.datatable import PostingDataTable


class ParamsTable(PostingDataTable):
    """
    The parameters table.
    """

    DEFAULT_CSS = """\
    ParamsTable {
        height: auto;
        width: 1fr;
        border-left: inner $accent 0%;
        margin-right: 1;

        &:focus {
            width: 1fr;
            border-left: inner $accent;
        }
    }
    """

    BINDINGS = [
        Binding("backspace", action="remove_row", description="Remove row"),
    ]

    @dataclass
    class Changed(Message):
        data_table: DataTable[str]

    def on_mount(self):
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.add_columns(*["Key", "Value"])

    def watch_has_focus(self, value: bool) -> None:
        self._scroll_cursor_into_view()
        return super().watch_has_focus(value)

    def add_row(
        self,
        *cells: str,
        height: int | None = 1,
        key: str | None = None,
        label: str | Text | None = None,
    ) -> RowKey:
        # TODO - this event was not bubbling up to the screen,
        # and I have no clue why. So I'll just post it directly
        # to the screen for now.
        self.screen.post_message(ParamsTable.Changed(self))
        return super().add_row(*cells, height=height, key=key, label=label)

    def remove_row(self, row_key: RowKey | str) -> None:
        self.screen.post_message(ParamsTable.Changed(self))
        return super().remove_row(row_key)

    def action_remove_row(self) -> None:
        try:
            cursor_cell_key = self.coordinate_to_cell_key(self.cursor_coordinate)
            cursor_row_key, _ = cursor_cell_key
            self.remove_row(cursor_row_key)
        except CellDoesNotExist:
            pass

    def as_dict(self) -> dict[str, str]:
        params: dict[str, str] = {}
        for row_index in range(self.row_count):
            row = self.get_row_at(row_index)
            params[row[0]] = row[1]
        return params


class QueryStringEditor(Vertical):
    """
    The query string editor.
    """

    # The CSS is very similar to the header editor
    DEFAULT_CSS = """\
    QueryStringEditor {
        & #param-inputs {
            height: auto;
            dock: bottom;
            & > Input {
                border: none;
                width: 1fr;
                margin-left: 1;

                &:focus {
                    background: $surface-lighten-1;
                }
            }

            #add-param-button {
                background: $success;
                color: $text;
                text-style: none;
                width: 10;
                margin: 0 1;
                &:hover {
                    text-style: b;
                    padding: 0 1;
                    border: none;
                    background: $success-darken-1;
                }
            }
        }
    }
    """

    def compose(self) -> ComposeResult:
        with Horizontal(id="param-inputs"):
            yield Input(placeholder="Key", id="param-key-input")
            yield Input(placeholder="Value", id="param-value-input")
            yield Button(label="Add param", disabled=True, id="add-param-button")

        yield ParamsTable()

    @on(Input.Changed, selector="#param-key-input")
    @on(Input.Changed, selector="#param-value-input")
    def determine_button_enabled(self) -> None:
        key_input = self.query_one("#param-key-input", Input)
        button = self.query_one("#add-param-button", Button)
        button.disabled = not key_input.value

    @on(Input.Submitted, selector="#param-key-input")
    @on(Input.Submitted, selector="#param-value-input")
    @on(Button.Pressed, selector="#add-param-button")
    def add_param(self, event: Input.Submitted | Button.Pressed) -> None:
        key_input = self.query_one("#param-key-input", Input)
        value_input = self.query_one("#param-value-input", Input)
        key = key_input.value
        value = value_input.value
        table = self.query_one(ParamsTable)

        def add_param() -> None:
            table.add_row(key, value)
            key_input.clear()
            value_input.clear()
            key_input.focus()
            table.move_cursor(row=table.row_count - 1)

        if key and value:
            add_param()
        elif key and not value:
            # This is a technically valid, but unlikely.
            if isinstance(event, Input.Submitted):
                input_id = event.input.id
                if input_id == "param-key-input":
                    value_input.focus()
                elif input_id == "param-value-input":
                    add_param()
        elif value and not key:
            key_input.focus()
        else:
            # Case where both are empty - do nothing.
            pass
