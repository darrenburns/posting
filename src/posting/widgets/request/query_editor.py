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
from posting.widgets.key_value import KeyValue


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

    DEFAULT_CSS = """\
    QueryStringEditor {
        & KeyValue {
            dock: bottom;
        }
    }
    """

    def compose(self) -> ComposeResult:
        yield KeyValue(
            Input(placeholder="Key", id="param-key-input"),
            Input(placeholder="Value", id="param-value-input"),
            button_label="Add parameter",
        )
        yield ParamsTable()

    @on(KeyValue.New)
    def add_header(self, event: KeyValue.New) -> None:
        table = self.query_one(ParamsTable)
        table.add_row(event.key, event.value)
        table.move_cursor(row=table.row_count - 1)
