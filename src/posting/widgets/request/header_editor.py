from dataclasses import dataclass

from rich.text import Text
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical, Horizontal
from textual.message import Message
from textual.widgets import Input, Button, DataTable
from textual.widgets.data_table import RowKey, CellDoesNotExist
from textual_autocomplete import DropdownItem, AutoComplete

from posting.datatable import PostingDataTable
from posting.request_headers import REQUEST_HEADERS


class HeaderEditor(Vertical):
    DEFAULT_CSS = """\
    #header-inputs {
        height: auto;
        width: 1fr;
        dock: bottom;
        & > Input {
            border: none;
            width: 1fr;
            margin-left: 1;
        }

        #add-header-button {
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
    """

    def compose(self) -> ComposeResult:
        with Horizontal(id="header-inputs"):
            yield Input(placeholder="Name", id="header-key-input")
            yield Input(placeholder="Value", id="header-value-input")
            add_header_button = Button(
                "Add header", disabled=True, id="add-header-button"
            )
            add_header_button.can_focus = False
            yield add_header_button
        yield HeadersTable()

    def on_mount(self):
        header_input = self.query_one("#header-key-input", Input)
        items: list[DropdownItem] = []
        for header in REQUEST_HEADERS:
            style = "yellow" if header["experimental"] else ""
            items.append(DropdownItem(main=Text(header["name"], style=style)))

        self.screen.mount(
            AutoComplete(
                header_input,
                items=items,
                prevent_default_tab=False,
                prevent_default_enter=False,
            )
        )

    @on(Input.Changed, selector="#header-key-input")
    @on(Input.Changed, selector="#header-value-input")
    def determine_button_enabled(self) -> None:
        # An HTTP header must have a key, but not necessarily a value.
        key_input = self.query_one("#header-key-input", Input)
        button = self.query_one("#add-header-button", Button)
        button.disabled = not key_input.value

    @on(Input.Submitted, selector="#header-key-input")
    @on(Input.Submitted, selector="#header-value-input")
    @on(Button.Pressed, selector="#add-header-button")
    def add_header(self, event: Input.Submitted | Button.Pressed) -> None:
        key_input = self.query_one("#header-key-input", Input)
        value_input = self.query_one("#header-value-input", Input)

        key = key_input.value
        value = value_input.value
        table = self.query_one(HeadersTable)

        def add_header() -> None:
            table.add_row(key, value)
            key_input.clear()
            value_input.clear()
            key_input.focus()
            table.move_cursor(row=table.row_count - 1)

        if key and value:
            add_header()
        elif key and not value:
            # This is a technically valid, but unlikely.
            if isinstance(event, Input.Submitted):
                input_id = event.input.id
                if input_id == "header-key-input":
                    value_input.focus()
                elif input_id == "header-value-input":
                    add_header()
            else:
                add_header()
        elif value and not key:
            key_input.focus()
        else:
            # Case where both are empty - do nothing.
            pass


class HeadersTable(PostingDataTable):
    """
    The headers table.
    """

    DEFAULT_CSS = """\
    HeadersTable {
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
        Binding("backspace", action="remove_header", description="Remove header"),
    ]

    @dataclass
    class Changed(Message):
        data_table: DataTable[str]

    def on_mount(self):
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.fixed_columns = 1
        self.add_columns(*["Header", "Value"])
        self.add_row("Content-Type", "application/json")

    def watch_has_focus(self, value: bool) -> None:
        self._scroll_cursor_into_view()
        return super().watch_has_focus(value)

    def as_dict(self) -> dict[str, str]:
        headers: dict[str, str] = {}
        for row_index in range(self.row_count):
            row = self.get_row_at(row_index)
            headers[row[0]] = row[1]
        return headers

    def action_cursor_down(self) -> None:
        if self.cursor_coordinate.row == self.row_count - 1:
            self.screen.focus_next()
        else:
            super().action_cursor_down()

    def action_cursor_up(self) -> None:
        if self.cursor_coordinate.row == 0:
            self.screen.focus_previous()
        else:
            super().action_cursor_up()

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
        self.screen.post_message(HeadersTable.Changed(self))
        return super().add_row(*cells, height=height, key=key, label=label)

    def remove_row(self, row_key: RowKey | str) -> None:
        self.screen.post_message(HeadersTable.Changed(self))
        return super().remove_row(row_key)

    def action_remove_header(self) -> None:
        try:
            cursor_cell_key = self.coordinate_to_cell_key(self.cursor_coordinate)
            cursor_row_key, _ = cursor_cell_key
            self.remove_row(cursor_row_key)
        except CellDoesNotExist:
            pass
