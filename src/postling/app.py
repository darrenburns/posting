from dataclasses import dataclass
from importlib.metadata import version
from pathlib import Path
import httpx
from textual import events, on
from textual.reactive import reactive
from textual.events import Message
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.screen import ModalScreen, Screen
from textual.widgets import (
    Button,
    DataTable,
    Footer,
    Input,
    Label,
    OptionList,
    TabPane,
    TabbedContent,
    TextArea,
)

from postling.highlight_url import URLHighlighter


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
        &:hover {
            background: $accent-darken-2;
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


class MethodSelectionPopup(ModalScreen[str]):
    CSS = """\
    MethodSelectionPopup {
        & > Vertical {
            height: auto;
            width: auto;
            margin: 4 3;
            border-left: outer $primary-lighten-1;
            & > OptionList {
                background: transparent;
                width: auto;
                border: none;
            }
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
        with Vertical():
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


class UrlInput(Input):
    """
    The URL input.
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
        border: none;

        &:focus {
            border: none;
        }

        &.empty {
            & .text-area--cursor-line {
                background: transparent;
            }
            & .text-area--cursor-gutter {
                background: transparent;
            }
        }

    }
    """

    def on_mount(self):
        self.show_line_numbers = True
        self.tab_behavior = "indent"
        self.set_class(len(self.text) == 0, "empty")
        return super().on_mount()

    @on(TextArea.Changed)
    def on_change(self, event: TextArea.Changed) -> None:
        self.set_class(len(self.text) == 0, "empty")


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


class HeadersTable(DataTable[str]):
    """
    The headers table.
    """

    DEFAULT_CSS = """\
    HeadersTable {
        height: auto;
        min-height: 5;
    }
    """

    def on_mount(self):
        self.show_header = False
        self.zebra_stripes = True
        self.add_columns(*["Key", "Value"])
        self.add_row("Content-Type", "application/json")
        self.add_row("Content-Type", "application/json")
        self.add_row("Content-Type", "application/json")


class RequestEditor(Vertical):
    """
    The request editor.
    """

    DEFAULT_CSS = """\
    """

    def compose(self) -> ComposeResult:
        with Vertical() as vertical:
            vertical.border_title = "Request"
            with TabbedContent():
                with TabPane("Headers"):
                    yield HeadersTable()
                with TabPane("Body"):
                    yield RequestBodyTextArea(language="json")
                with TabPane("Parameters"):
                    yield DataTable()

    def on_mount(self):
        self.border_title = "Request"
        self.add_class("section")


class MainScreen(Screen[None]):
    BINDINGS = [
        Binding("escape", "app.quit", "Quit"),
        Binding("ctrl+j", "send_request", "Send request"),
        Binding("ctrl+t", "change_method", "Change method"),
        Binding("ctrl+n", "tree", "DEBUG Show tree"),
    ]

    selected_method = reactive("GET")

    def compose(self) -> ComposeResult:
        yield AppHeader(f"[b]Postling[/] [white dim]{version('postling')}[/]")
        yield UrlBar()
        with AppBody():
            yield RequestEditor()
            yield ResponseTextArea(language="json", read_only=True)
        yield Footer()

    @on(SendRequestButton.Pressed)
    @on(UrlInput.Submitted)
    def send_request(self) -> None:
        try:
            with httpx.Client() as client:
                # TODO - update the request object here.
                # TODO - think about whether we store a single request instance or create a new one each time.
                request = self.build_httpx_request()
                response = client.send(request=request)
        except Exception:
            pass
        else:
            self.response_text_area.text = response.text

    def action_send_request(self) -> None:
        self.send_request()

    def action_change_method(self) -> None:
        self.method_selection()

    def action_tree(self) -> None:
        from textual import log

        log.info(self.app.tree)

    @on(MethodSelection.Clicked)
    def method_selection(self) -> None:
        def set_method(method: str) -> None:
            self.selected_method = method

        self.app.push_screen(MethodSelectionPopup(), callback=set_method)

    def build_httpx_request(self) -> httpx.Request:
        return httpx.Request(
            method="GET",
            url=self.url_input.value,
            content=self.request_body_text_area.text,
        )

    @property
    def url_input(self) -> UrlInput:
        return self.query_one(UrlInput)

    @property
    def response_text_area(self) -> ResponseTextArea:
        return self.query_one(ResponseTextArea)

    @property
    def request_body_text_area(self) -> RequestBodyTextArea:
        return self.query_one(RequestBodyTextArea)

    def watch_selected_method(self, value: str) -> None:
        self.query_one(MethodSelection).set_method(value)


class Postling(App[None]):
    ENABLE_COMMAND_PALETTE = False
    CSS_PATH = Path(__file__).parent / "postling.scss"

    def get_default_screen(self) -> MainScreen:
        return MainScreen()


app = Postling()
if __name__ == "__main__":
    app.run()
