from textual.app import ComposeResult
from textual.containers import Horizontal
from textual.widgets import Input, Button

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
    }
    """

    def on_mount(self):
        self.highlighter = URLHighlighter()


class SendRequestButton(Button, can_focus=False):
    """
    The button for sending the request.
    """

    DEFAULT_CSS = """\
    SendRequestButton {
        padding: 0 1;
        height: 1;
        min-width: 10;
        background: $success;
        color: $text;
        border: none;
        text-style: none;
        &:hover {
            text-style: b;
            padding: 0 1;
            border: none;
            background: $success;
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

    def compose(self) -> ComposeResult:
        with Horizontal():
            yield MethodSelection("GET")
            yield UrlInput(
                "http://jsonplaceholder.typicode.com/posts",
                placeholder="Enter a URL...",
            )
            yield SendRequestButton("Send")
