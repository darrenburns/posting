from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal
from textual.widgets import Input, Button
from textual_autocomplete import AutoComplete, DropdownItem
from textual_autocomplete._autocomplete2 import TargetState

from posting.highlight_url import URLHighlighter
from posting.widgets.request.method_selection import MethodSelection


class UrlInput(Input):
    """
    The URL input.
    """

    DEFAULT_CSS = """\
    UrlInput {
        border: none;
        width: 1fr;
        &:focus {
            border: none;
            padding: 0 1;
            & .input--cursor {
              color: $text;
              background: $accent-lighten-2;
            }
        }
        &.error {
            border-left: thick $error;
        }
    }
    """

    BINDINGS = [
        Binding("down", "app.focus_next", "Focus next"),
    ]

    def on_mount(self):
        self.highlighter = URLHighlighter()

    @on(Input.Changed)
    def on_change(self, event: Input.Changed) -> None:
        self.remove_class("error")


class SendRequestButton(Button, can_focus=False):
    """
    The button for sending the request.
    """

    DEFAULT_CSS = """\
    SendRequestButton {
        padding: 0 1;
        height: 1;
        min-width: 10;
        background: $primary;
        color: $text;
        border: none;
        text-style: none;
        &:hover {
            text-style: b;
            padding: 0 1;
            border: none;
            background: $primary-darken-1;
        }
    }
    """


class UrlBar(Horizontal):
    """
    The URL bar.
    """

    DEFAULT_CSS = """\
    UrlBar {
        height: 1;
        padding: 0 3;
    }
    """

    def __init__(
        self,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(name=name, id=id, classes=classes, disabled=disabled)
        self.cached_base_urls: list[str] = []

    def compose(self) -> ComposeResult:
        yield MethodSelection("GET")
        yield UrlInput(
            placeholder="Enter a URL...",
            id="url-input",
        )
        yield SendRequestButton("Send")

    def on_mount(self) -> None:
        self.auto_complete = AutoComplete(
            target=self.query_one("#url-input", UrlInput),
            items=self._get_autocomplete_items,
        )
        self.screen.mount(self.auto_complete)

    def _get_autocomplete_items(self, target_state: TargetState) -> list[DropdownItem]:
        return [DropdownItem(main=base_url) for base_url in self.cached_base_urls]
