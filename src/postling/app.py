from importlib.metadata import version
from pathlib import Path
import httpx
from textual import events, on
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.screen import ModalScreen, Screen
from textual.widgets import Button, Footer, Input, Label, OptionList, TextArea


class AppHeader(Label):
    """The header of the app."""

    DEFAULT_CSS = """\
    AppHeader {
        color: $accent-lighten-2;
        padding: 1 3;
    }
    """


class AppBody(Vertical):
    """The body of the app."""

    DEFAULT_CSS = """\
    AppBody {
        padding: 1 2;
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

    @on(events.Click)
    def open_method_selection_popup(self, event: events.Click) -> None:
        self.app.push_screen(MethodSelectionPopup())


class MethodSelectionPopup(ModalScreen[str]):
    def compose(self) -> ComposeResult:
        yield OptionList(*["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"])

    @on(OptionList.OptionSelected)
    def return_selected_method(self, event: OptionList.OptionSelected) -> None:
        self.dismiss(event.option.prompt)


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


class SendRequestButton(Button, can_focus=False):
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
        padding: 0 3;
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
    }
    """

    def on_mount(self):
        self.border_title = "Request body"
        self.add_class("section")
        return super().on_mount()


class ResponseTextArea(TextArea):
    """
    For displaying responses.
    """

    DEFAULT_CSS = """\
    ResponseTextArea {
    }
    """

    def on_mount(self):
        self.border_title = "Response body"
        self.add_class("section")
        return super().on_mount()


class MainScreen(Screen[None]):
    BINDINGS = [
        Binding("escape", "app.quit", "Quit"),
        Binding("ctrl+j", "send_request", "Send request"),
        Binding("ctrl+t", "change_method", "Change method"),
    ]

    def on_mount(self) -> None:
        self.request = httpx.Request(
            method="GET", url="http://jsonplaceholder.typicode.com/posts"
        )

    def compose(self) -> ComposeResult:
        yield AppHeader(f"[b]Postling[/] {version('postling')}")
        yield UrlBar()
        with AppBody():
            yield RequestBodyTextArea(language="json")
            yield ResponseTextArea(language="json", read_only=True)
        yield Footer()

    @on(SendRequestButton.Pressed)
    @on(UrlInput.Submitted)
    def send_request(self) -> None:
        try:
            with httpx.Client() as client:
                response = client.send(self.request)
        except Exception:
            pass
        else:
            self.response_text_area.text = response.text

    def action_send_request(self) -> None:
        self.send_request()

    def action_change_method(self) -> None:
        self.app.push_screen(MethodSelectionPopup())

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
