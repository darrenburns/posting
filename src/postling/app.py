from importlib.metadata import version
from pathlib import Path
import httpx
from textual import on
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal
from textual.screen import Screen
from textual.widgets import Button, Footer, Input, Label, TextArea


class AppHeader(Label):
    """The header of the app."""

    DEFAULT_CSS = """\
    AppHeader {
        color: $accent-lighten-2;
        padding: 1 3;
    }
    """


class MethodSelection(Label):
    """
    The label for the URL bar.
    """

    DEFAULT_CSS = """\
    MethodSelection {
        padding: 0 1;
        background: $accent-darken-1;
        color: $text;
    }
    """


class UrlInput(Input):
    """
    The URL bar.
    """

    DEFAULT_CSS = """\
    UrlInput {
        padding: 0 1;
        border: none;
        height: 1;
        width: 1fr;
        &:focus {
            border: none;
            padding: 0 1;
        }
    }
    """


class SendRequestButton(Button):
    """
    The button for sending the request.
    """

    DEFAULT_CSS = """\
    SendRequestButton {
        padding: 0 1;
        height: 1;
        background: $success;
        color: $text;
        border: none;
        &:hover {
            padding: 0 1;
            border: none;
            background: $success-darken-1;
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
    }
    """

    def compose(self) -> ComposeResult:
        with Horizontal():
            yield MethodSelection("GET")
            yield UrlInput(
                "http://jsonplaceholder.typicode.com/posts",
                placeholder="http://jsonplaceholder.typicode.com/posts",
            )
            yield SendRequestButton("Send")


class RequestBodyTextArea(TextArea):
    """
    For editing request bodies.
    """

    DEFAULT_CSS = """\
    RequestBodyTextArea {
        border: round $primary 80%;
        &:focus {
            border: round $primary-lighten-2;
        }
    }
    """


class ResponseTextArea(TextArea):
    """
    For displaying responses.
    """

    DEFAULT_CSS = """\
    ResponseTextArea {
        border: round $primary 80%;
        &:focus {
            border: round $primary-lighten-2;
        }
    }
    """


class MainScreen(Screen[None]):
    BINDINGS = [
        Binding("escape", "app.quit", "Quit"),
        Binding("ctrl+j", "send_request", "Send request"),
    ]

    def compose(self) -> ComposeResult:
        yield AppHeader(f"[b]Postling[/] {version('postling')}")
        yield UrlBar()
        yield RequestBodyTextArea(language="json")
        yield ResponseTextArea(language="json", read_only=True)
        yield Footer()

    @on(SendRequestButton.Pressed)
    @on(UrlInput.Submitted)
    def send_request(self) -> None:
        try:
            response = httpx.get(self.url_input.value)
        except Exception:
            pass
        else:
            self.response_text_area.text = response.text

    def action_send_request(self) -> None:
        self.send_request()

    @property
    def url_input(self) -> UrlInput:
        return self.query_one(UrlInput)

    @property
    def response_text_area(self) -> ResponseTextArea:
        return self.query_one(ResponseTextArea)


class Postling(App[None]):
    ENABLE_COMMAND_PALETTE = False
    CSS_PATH = Path(__file__).parent / "postling.scss"

    def get_default_screen(self) -> MainScreen:
        return MainScreen()


app = Postling()
if __name__ == "__main__":
    app.run()
