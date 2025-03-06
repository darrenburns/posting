from dataclasses import dataclass, field
from typing import Any, Iterable, Self
from rich.style import Style
from rich.text import Text
from textual import on
from textual.app import RenderResult
from textual.binding import Binding
from textual.color import Color
from textual.coordinate import Coordinate
from textual.filter import DimFilter
from textual.message import Message
from textual.message_pump import MessagePump
from textual.strip import Strip
from textual.widgets import DataTable
from textual.widgets.data_table import CellDoesNotExist, CellKey, RowKey


class PostingDataTable(DataTable[str | Text]):
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

    def __init__(self, *args: Any, **kwargs: Any):
        super().__init__(*args, **kwargs)
        self.cursor_vertical_escape = True
        """The cursor can escape the table by pressing up when it's at the top (will focus previous widget in focus chain)."""
        self.row_disable = False
        """If True, rows will have a checkbox added to them and can be disabled with space bar."""

    @dataclass
    class Checkbox:
        """A checkbox, added to rows to make them enable/disable."""

        data_table: "PostingDataTable"
        checked: bool = True
        text: Text = field(default_factory=lambda: Text("✔︎"))

        def __rich__(self) -> RenderResult:
            return self.text

        def toggle(self) -> bool:
            """Toggle the checkbox."""
            self.checked = not self.checked
            self.text = Text("✔︎" if self.checked else " ")
            return self.checked

        @property
        def plain(self) -> str:
            return self.text.plain

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
        *cells: str | Text,
        height: int | None = 1,
        key: str | None = None,
        label: str | Text | None = None,
        explicit_by_user: bool = True,
        sender: MessagePump | None = None,
    ) -> RowKey:
        msg = self.RowsAdded(self, explicit_by_user=explicit_by_user)
        if sender:
            msg.set_sender(sender)
        self.post_message(msg)
        if self.row_disable and label is None:
            label = self.Checkbox(self, True)
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

    def replace_all_rows(
        self, rows: Iterable[Iterable[str]], enable_states: Iterable[bool] | None = None
    ) -> None:
        self.clear()
        if self.row_disable and enable_states:
            for row, enable in zip(rows, enable_states):
                self.add_row(
                    *row,
                    explicit_by_user=False,
                    label=self.Checkbox(self, enable),
                )
        else:
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

    def action_cursor_down(self) -> None:
        self._set_hover_cursor(False)
        if (
            self.cursor_coordinate.row == self.row_count - 1
            and self.cursor_vertical_escape
        ):
            self.screen.focus_next()
        else:
            cursor_type = self.cursor_type
            if self.show_cursor and (cursor_type == "cell" or cursor_type == "row"):
                row, column = self.cursor_coordinate
                if row == self.row_count - 1:
                    self.cursor_coordinate = Coordinate(0, column)
                else:
                    self.cursor_coordinate = self.cursor_coordinate.down()
            else:
                super().action_cursor_down()

    def action_cursor_up(self) -> None:
        self._set_hover_cursor(False)
        if self.cursor_coordinate.row == 0 and self.cursor_vertical_escape:
            self.screen.focus_previous()
        else:
            cursor_type = self.cursor_type
            if self.show_cursor and (cursor_type == "cell" or cursor_type == "row"):
                row, column = self.cursor_coordinate
                if row == 0:
                    self.cursor_coordinate = Coordinate(self.row_count - 1, column)
                else:
                    self.cursor_coordinate = self.cursor_coordinate.up()
            else:
                super().action_cursor_up()

    @on(RowsRemoved)
    @on(RowsAdded)
    def _on_rows_removed(self, event: RowsRemoved | RowsAdded) -> None:
        self.set_class(self.row_count == 0, "empty")

    def action_remove_row(self) -> None:
        try:
            cursor_cell_key = self.coordinate_to_cell_key(self.cursor_coordinate)
            cursor_row_key, _ = cursor_cell_key
            self.remove_row(cursor_row_key)
        except CellDoesNotExist:
            pass

    def action_toggle_row(self) -> None:
        try:
            cursor_cell_key = self.coordinate_to_cell_key(self.cursor_coordinate)
            cursor_row_key, _ = cursor_cell_key
            self.toggle_row(cursor_row_key)
        except CellDoesNotExist:
            pass

    def toggle_row(self, row_key: RowKey) -> None:
        try:
            checkbox: PostingDataTable.Checkbox = self.rows[row_key].label
        except KeyError:
            return
        else:
            checkbox.toggle()
            # HACK: Without such increment, the table is refreshed
            # only when focus changes to another column.
            self._update_count += 1
            self.refresh()

    def is_row_enabled_at(self, row_index: int) -> bool:
        row_key = self._row_locations.get_key(row_index)
        try:
            checkbox: PostingDataTable.Checkbox = self.rows[row_key].label
        except KeyError:
            return True
        else:
            return checkbox.checked

    def render_line(self, y: int) -> Strip:
        strip = super().render_line(y)
        try:
            row_key, _ = self._get_offsets(y)
        except LookupError:
            return Strip.blank(self.size.width)

        try:
            label = self.rows[row_key].label
        except KeyError:
            return strip

        if label is None:
            return strip

        is_disabled = not label.checked
        if self.row_disable and is_disabled:
            strip = strip.apply_style(Style(dim=True))
            strip = strip.apply_filter(DimFilter(), Color(0, 0, 0))
        return strip

    @on(DataTable.RowLabelSelected)
    def _on_row_label_selected(self, event: DataTable.RowLabelSelected) -> None:
        if self.row_disable:
            event.prevent_default()
            self.toggle_row(event.row_key)

    def __rich_repr__(self):
        yield "id", self.id
        yield "classes", self.classes
        yield "row_count", self.row_count
