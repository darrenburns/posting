from typing import TypeVar
from textual.binding import Binding
from textual.widgets import Select
from textual.widgets._select import SelectOverlay

T = TypeVar("T")


class PostingSelect(Select[T], inherit_bindings=False):
    BINDINGS = [
        Binding("enter,space,l", "show_overlay", "Show Overlay", show=False),
        Binding("up,k", "cursor_up", "Cursor Up", show=False),
        Binding("down,j", "cursor_down", "Cursor Down", show=False),
    ]

    def action_cursor_up(self):
        if self.expanded:
            self.select_overlay.action_cursor_up()
        else:
            self.screen.focus_previous()

    def action_cursor_down(self):
        if self.expanded:
            self.select_overlay.action_cursor_down()
        else:
            self.screen.focus_next()

    @property
    def select_overlay(self) -> SelectOverlay:
        return self.query_one(SelectOverlay)
