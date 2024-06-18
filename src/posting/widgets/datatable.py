from dataclasses import dataclass
from typing import Iterable, Self
from rich.text import Text
from textual import on
from textual.binding import Binding
from textual.message import Message
from textual.widgets import DataTable
from textual.widgets.data_table import CellKey, RowKey


class PostingDataTable(DataTable[str]):
    DEFAULT_CSS = """\
PostingDataTable {
    &.empty {
        display: none;
    }
}
"""

    BINDINGS = [
        Binding("up,k", "cursor_up", "Cursor Up", show=False),
        Binding("down,j", "cursor_down", "Cursor Down", show=False),
        Binding("right,l", "cursor_right", "Cursor Right", show=False),
        Binding("left,h", "cursor_left", "Cursor Left", show=False),
        Binding("f", "toggle_fixed_columns", "Toggle Fixed Column", show=False),
        Binding("home", "scroll_home", "Home", show=False),
        Binding("end", "scroll_end", "End", show=False),
        Binding("g,ctrl+home", "scroll_top", "Top", show=False),
        Binding("G,ctrl+end", "scroll_bottom", "Bottom", show=False),
    ]

    @dataclass
    class RowsRemoved(Message):
        data_table: "PostingDataTable"
        explicit_by_user: bool = True

        @property
        def control(self) -> "PostingDataTable":
            return self.data_table

    @dataclass
    class RowsAdded(Message):
        data_table: "PostingDataTable"
        explicit_by_user: bool = True

        @property
        def control(self) -> "PostingDataTable":
            return self.data_table

    def add_row(
        self,
        *cells: str,
        height: int | None = 1,
        key: str | None = None,
        label: str | Text | None = None,
        explicit_by_user: bool = True,
    ) -> RowKey:
        self.post_message(self.RowsAdded(self, explicit_by_user=explicit_by_user))
        return super().add_row(*cells, height=height, key=key, label=label)

    def action_toggle_fixed_columns(self) -> None:
        self.fixed_columns = 1 if self.fixed_columns == 0 else 0

    def remove_row(self, row_key: RowKey | str) -> None:
        self.post_message(self.RowsRemoved(self))
        rv = super().remove_row(row_key)
        self._column_width_refresh()
        return rv

    def clear(self, columns: bool = False) -> Self:
        self.post_message(self.RowsRemoved(self, explicit_by_user=False))
        super().clear(columns=columns)
        self._column_width_refresh()
        return self

    def replace_all_rows(self, rows: Iterable[Iterable[str]]) -> None:
        self.clear()
        for row in rows:
            self.add_row(*row, explicit_by_user=False)
        self._column_width_refresh()

    def _column_width_refresh(self) -> None:
        # TODO - fix this inside Textual.
        if self.row_count > 0:
            row_zero = list(self._data.keys())[0]
            columns = set(list(self._data.values())[0].keys())
            self._update_column_widths(
                {CellKey(row_zero, column) for column in columns}
            )

    def action_cursor_up(self) -> None:
        row, _ = self.cursor_coordinate
        if row == 0:
            self.screen.focus_previous()
        else:
            super().action_cursor_up()

    @on(RowsRemoved)
    @on(RowsAdded)
    def _on_rows_removed(self, event: RowsRemoved | RowsAdded) -> None:
        self.set_class(self.row_count == 0, "empty")

    def __rich_repr__(self):
        yield "id", self.id
        yield "classes", self.classes
        yield "row_count", self.row_count
