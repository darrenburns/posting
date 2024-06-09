from dataclasses import dataclass
from rich.text import Text
from textual.binding import Binding
from textual.message import Message
from textual.widgets import DataTable
from textual.widgets.data_table import CellKey, RowKey


class PostingDataTable(DataTable[str]):
    BINDINGS = [
        Binding("up,k", "cursor_up", "Cursor Up", show=False),
        Binding("down,j", "cursor_down", "Cursor Down", show=False),
        Binding("right,l", "cursor_right", "Cursor Right", show=False),
        Binding("left,h", "cursor_left", "Cursor Left", show=False),
        Binding("f", "toggle_fixed_columns", "Toggle Fixed Column", show=False),
        Binding("g,home", "scroll_home", "Home", show=False),
        Binding("G,end", "scroll_end", "End", show=False),
    ]

    @dataclass
    class Changed(Message):
        data_table: "PostingDataTable"

        @property
        def control(self) -> "PostingDataTable":
            return self.data_table

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
        self.post_message(self.Changed(self))
        return super().add_row(*cells, height=height, key=key, label=label)

    def action_toggle_fixed_columns(self) -> None:
        self.fixed_columns = 1 if self.fixed_columns == 0 else 0

    def remove_row(self, row_key: RowKey | str) -> None:
        self.post_message(self.Changed(self))
        rv = super().remove_row(row_key)

        # TODO - fix this inside Textual.
        if self.row_count > 0:
            row_zero = list(self._data.keys())[0]
            columns = set(list(self._data.values())[0].keys())
            self._update_column_widths(
                {CellKey(row_zero, column) for column in columns}
            )
        return rv
