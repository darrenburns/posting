from typing import TypeVar
from textual.binding import Binding
from textual.widgets import Select
from textual.widgets._select import SelectOverlay, SelectCurrent
T = TypeVar("T")


class PostingSelectOverlay(SelectOverlay, inherit_bindings=False):

    BINDINGS = [
        Binding("enter", "select", "Show Overlay", show=False),
        Binding("up", "cursor_up", "Cursor Up", show=False),
        Binding("escape", "dismiss"),
        Binding("down", "cursor_down", "Cursor Down", show=False),
    ]

    async def _on_key(self, event):
        should_stop_event = False
        if event.key == "k":
            self.action_cursor_up()
            should_stop_event = True
        elif event.key == "j":
            self.action_cursor_down()
            should_stop_event = True
        elif event.key == "l" or event.key == "space":
            self.action_select()
            should_stop_event = True

        # stops type_to_search from recieveng the event, as well as the select
        if should_stop_event:
            event.stop()
            event.prevent_default()

    def __init__(self, type_to_search):
        super().__init__(type_to_search=type_to_search)


class PostingSelect(Select[T], inherit_bindings=False):
    BINDINGS = [
        Binding("enter,space,l", "show_overlay", "Show Overlay", show=False),
        Binding("up,k", "cursor_up", "Cursor Up", show=False),
        Binding("down,j", "cursor_down", "Cursor Down", show=False),
    ]

    def action_cursor_up(self):
        self.screen.focus_previous()

    def select_or_highlight(self, entry: str):
        if self.expanded:
            overlay = self.select_overlay
            options = self._options
            for index, (_prompt, option_value) in enumerate(options):
                if option_value == entry:
                    overlay.highlighted = index
                    return
        else:
            self.value = entry

    def action_cursor_down(self):
        self.screen.focus_next()

    def compose(self):
        yield SelectCurrent(self.prompt)
        yield PostingSelectOverlay(self._type_to_search).data_bind(
            compact=Select.compact
        )

    @property
    def select_overlay(self) -> PostingSelectOverlay:
        result: SelectOverlay = self.query_one(PostingSelectOverlay)
        return result
