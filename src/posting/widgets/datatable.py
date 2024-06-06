from textual.binding import Binding
from textual.screen import ModalScreen
from textual.widgets import DataTable


class PostingDataTable(DataTable[str]):
    BINDINGS = [
        Binding("up,k", "cursor_up", "Cursor Up", show=False),
        Binding("down,j", "cursor_down", "Cursor Down", show=False),
        Binding("right,l", "cursor_right", "Cursor Right", show=False),
        Binding("left,h", "cursor_left", "Cursor Left", show=False),
        Binding("f", "toggle_fixed_columns", "Toggle Fixed Column", show=False),
    ]

    def action_toggle_fixed_columns(self) -> None:
        self.fixed_columns = 1 if self.fixed_columns == 0 else 0
