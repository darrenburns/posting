from rich.text import Text
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.widgets import Input
from textual.widgets.data_table import CellDoesNotExist
from textual_autocomplete import DropdownItem, AutoComplete
from posting.collection import Header

from posting.widgets.datatable import PostingDataTable
from posting.request_headers import REQUEST_HEADERS
from posting.widgets.key_value import KeyValueEditor, KeyValueInput


class HeaderEditor(Vertical):
    def compose(self) -> ComposeResult:
        yield KeyValueEditor(
            HeadersTable(),
            KeyValueInput(
                Input(placeholder="Name", id="header-key-input"),
                Input(placeholder="Value", id="header-value-input"),
                button_label="Add header",
            ),
            empty_message="There are no headers.",
        )

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

    def on_mount(self):
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.fixed_columns = 1
        self.add_columns(*["Header", "Value"])

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

    def action_remove_header(self) -> None:
        try:
            cursor_cell_key = self.coordinate_to_cell_key(self.cursor_coordinate)
            cursor_row_key, _ = cursor_cell_key
            self.remove_row(cursor_row_key)
        except CellDoesNotExist:
            pass

    def to_model(self) -> list[Header]:
        headers: list[Header] = []
        # TODO - handle enabled/disabled...
        for row_index in range(self.row_count):
            row = self.get_row_at(row_index)
            headers.append(Header(name=row[0], value=row[1], enabled=True))
        return headers
