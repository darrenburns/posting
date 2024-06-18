from dataclasses import dataclass

from textual import events, on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.message import Message
from textual.screen import ModalScreen
from textual.widgets import OptionList, Footer, Label


class MethodSelectionPopup(ModalScreen[str]):
    CSS = """\
    MethodSelectionPopup {
        & > OptionList {
            margin: 4 3;
            width: 10;
            height: 7;
            border: none;
            border-left: outer $primary-lighten-1;
        }
    }"""

    BINDINGS = [
        Binding("escape", "app.pop_screen", "Dismiss"),
        Binding("g", "dismiss_with_http_method('GET')", "GET"),
        Binding("p", "dismiss_with_http_method('POST')", "POST"),
        Binding("a", "dismiss_with_http_method('PATCH')", "PATCH"),
        Binding("u", "dismiss_with_http_method('PUT')", "PUT"),
        Binding("d", "dismiss_with_http_method('DELETE')", "DELETE"),
        Binding("o", "dismiss_with_http_method('OPTIONS')", "OPTIONS"),
        Binding("h", "dismiss_with_http_method('HEAD')", "HEAD"),
    ]

    def compose(self) -> ComposeResult:
        yield OptionList("GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS")
        yield Footer()

    def on_click(self, event: events.Click) -> None:
        if self.get_widget_at(event.screen_x, event.screen_y)[0] is self:
            self.dismiss()

    @on(OptionList.OptionSelected)
    def return_selected_method(self, event: OptionList.OptionSelected) -> None:
        self.action_dismiss_with_http_method(event.option.prompt)

    def action_dismiss_with_http_method(self, method: str) -> None:
        self.dismiss(method)


class MethodSelection(Label):
    """
    The label for the URL bar.
    """

    DEFAULT_CSS = """\
    MethodSelection {
        padding: 0 1;
        background: $secondary;
        color: $text;
        &:hover {
            background: $secondary-darken-1;
        }
    }
    """

    @dataclass
    class Clicked(Message):
        """Posted when the method selection label is clicked."""

    @on(events.Click)
    def open_method_selection_popup(self, event: events.Click) -> None:
        self.post_message(MethodSelection.Clicked())

    def set_method(self, method: str) -> None:
        self.renderable = f"{method}"
        self.refresh(layout=True)
