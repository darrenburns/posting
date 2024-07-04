from typing import TypeVar
from textual.binding import Binding
from textual.widgets import Select
from textual.widgets._select import SelectOverlay

T = TypeVar("T")


class PostingSelect(Select[T], inherit_bindings=False):
    BINDINGS = [
        Binding("enter,space,l", "show_overlay"),
        Binding("up,k", "cursor_up"),
        Binding("down,j", "cursor_down"),
    ]

    def action_cursor_up(self):
        self.select_overlay.action_cursor_up()

    def action_cursor_down(self):
        self.select_overlay.action_cursor_down()

    @property
    def select_overlay(self) -> SelectOverlay:
        return self.query_one(SelectOverlay)
