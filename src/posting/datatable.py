from textual.binding import Binding
from textual.widgets import DataTable


class PostingDataTable(DataTable[str]):
    BINDINGS = [
        Binding("up,k", "cursor_up", "Cursor Up", show=False),
        Binding("down,j", "cursor_down", "Cursor Down", show=False),
        Binding("right,l", "cursor_right", "Cursor Right", show=False),
        Binding("left,h", "cursor_left", "Cursor Left", show=False),
    ]

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
