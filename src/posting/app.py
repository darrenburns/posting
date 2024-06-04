from importlib.metadata import version
from itertools import cycle
from pathlib import Path
from typing import Literal
import httpx
from textual import on
from textual.design import ColorSystem
from textual.reactive import Reactive, reactive
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.screen import Screen
from textual.widgets import (
    Button,
    Footer,
    Input,
    Label,
    TextArea,
)
from textual.widgets._tabbed_content import ContentTab

from posting.commands import PostingProvider
from posting.widgets.request.header_editor import HeadersTable
from posting.messages import HttpResponseReceived
from posting.widgets.request.method_selection import MethodSelectionPopup, MethodSelection

from posting.widgets.request.query_editor import ParamsTable

from posting.widgets.request.request_body import RequestBodyTextArea
from posting.widgets.request.request_editor import RequestEditor
from posting.widgets.request.url_bar import UrlInput, UrlBar
from posting.widgets.response.response_area import ResponseArea


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
        padding: 1 2 0 2;
    }
    """


class MainScreen(Screen[None]):
    BINDINGS = [
        Binding("ctrl+j", "send_request", "Send request"),
        Binding("ctrl+t", "change_method", "Change method"),
        Binding("ctrl+i", "focus_headers", "Headers"),
        # Binding("ctrl+n", "tree", "DEBUG Show tree"),
    ]

    selected_method = reactive("GET")
    layout: Reactive[Literal["horizontal", "vertical"]] = reactive("vertical")

    def compose(self) -> ComposeResult:
        yield AppHeader(f"[i]Posting[/] [white dim]{version('posting')}[/]")
        yield UrlBar()
        with AppBody():
            yield RequestEditor()
            yield ResponseArea()
        yield Footer()

    @on(Button.Pressed, selector="SendRequestButton")
    @on(Input.Submitted, selector="UrlInput")
    async def send_request(self) -> None:
        try:
            async with httpx.AsyncClient() as client:
                # TODO - update the request object here.
                # TODO - think about whether we store a single request instance or create a new one each time.
                request = self.build_httpx_request()
                print("-- sending request --")
                print(request)
                print(request.headers)
                response = await client.send(request=request)
                self.post_message(HttpResponseReceived(response))

        except Exception:
            pass

    @on(HttpResponseReceived)
    def on_response_received(self, event: HttpResponseReceived) -> None:
        # TODO - call method on the response section
        self.response_area.response = event.response

    async def action_send_request(self) -> None:
        await self.send_request()

    def action_change_method(self) -> None:
        self.method_selection()

    def action_focus_headers(self) -> None:
        self.headers_table.focus()

    def watch_layout(self, layout: Literal["horizontal", "vertical"]) -> None:
        classes = {"horizontal", "vertical"}
        other_class = classes.difference({layout}).pop()
        self.app_body.add_class(f"layout-{layout}")
        self.app_body.remove_class(f"layout-{other_class}")

    # def action_tree(self) -> None:
    #     from textual import log

    #     log.info(self.app.tree)
    #     log(self.app.get_css_variables())
    #     self.app.next_theme()

    @on(TextArea.Changed, selector="RequestBodyTextArea")
    def on_request_body_change(self, event: TextArea.Changed) -> None:
        body_tab = self.query_one("#--content-tab-body-pane", ContentTab)
        if event.text_area.text:
            body_tab.update("Body[cyan b]•[/]")
        else:
            body_tab.update("Body")

    @on(HeadersTable.Changed)
    def on_content_changed(self, event: HeadersTable.Changed) -> None:
        print("on_content_changed")
        headers_tab = self.query_one("#--content-tab-headers-pane", ContentTab)
        print("event.data_table.row_count", event.data_table.row_count)
        if event.data_table.row_count:
            headers_tab.update("Headers[cyan b]•[/]")
        else:
            headers_tab.update("Headers")

    @on(ParamsTable.Changed)
    def on_params_changed(self, event: ParamsTable.Changed) -> None:
        params_tab = self.query_one("#--content-tab-parameters-pane", ContentTab)
        if event.data_table.row_count:
            params_tab.update("Parameters[cyan b]•[/]")
        else:
            params_tab.update("Parameters")

    @on(MethodSelection.Clicked)
    def method_selection(self) -> None:
        def set_method(method: str) -> None:
            self.selected_method = method

        self.app.push_screen(MethodSelectionPopup(), callback=set_method)

    def build_httpx_request(self) -> httpx.Request:
        return httpx.Request(
            method=self.selected_method,
            url=self.url_input.value,
            params=self.params_table.as_dict(),
            content=self.request_body_text_area.text,
            headers=self.headers_table.as_dict(),
        )

    @property
    def url_input(self) -> UrlInput:
        return self.query_one(UrlInput)

    @property
    def response_area(self) -> ResponseArea:
        return self.query_one(ResponseArea)

    @property
    def request_body_text_area(self) -> RequestBodyTextArea:
        return self.query_one(RequestBodyTextArea)

    @property
    def headers_table(self) -> HeadersTable:
        return self.query_one(HeadersTable)

    @property
    def params_table(self) -> ParamsTable:
        return self.query_one(ParamsTable)

    @property
    def app_body(self) -> AppBody:
        return self.query_one(AppBody)

    def watch_selected_method(self, value: str) -> None:
        self.query_one(MethodSelection).set_method(value)


class Posting(App[None]):
    COMMANDS = {PostingProvider}
    CSS_PATH = Path(__file__).parent / "posting.scss"
    BINDINGS = [
        Binding(
            "ctrl+p",
            "command_palette",
            description="Commands",
            show=True,
        ),
    ]

    themes: dict[str, ColorSystem] = {
        "textual": ColorSystem(
            primary="#004578",
            secondary="#ffa62b",
            warning="#ffa62b",
            error="#ba3c5b",
            success="#4EBF71",
            accent="#0178D4",
            dark=True,
        ),
        "sunset": ColorSystem(
            primary="#ff4500",
            secondary="#ff8c00",
            warning="#ff6347",
            error="#b22222",
            success="#32cd32",
            accent="#ffd700",
            dark=True,
        ),
        "ocean": ColorSystem(
            primary="#1e90ff",
            secondary="#00ced1",
            warning="#ffa07a",
            error="#ff4500",
            success="#20b2aa",
            accent="#4682b4",
            dark=True,
        ),
    }

    theme: Reactive[str | None] = reactive(None)

    def __init__(self):
        super().__init__()
        self.theme = "textual"
        self.themes_cycle = cycle(self.themes.items())

    def get_default_screen(self) -> MainScreen:
        self.main_screen = MainScreen()
        return self.main_screen

    def get_css_variables(self) -> dict[str, str]:
        if self.theme:
            system = self.themes.get(self.theme)
            if system:
                theme = system.generate()
            else:
                theme = {}
        else:
            theme = {}
        return {**super().get_css_variables(), **theme}

    def command_layout(self, layout: Literal["vertical", "horizontal"]) -> None:
        self.main_screen.layout = layout

    def command_theme(self, theme: str) -> None:
        self.theme = theme
        self.refresh_css()
        self.notify(f"Theme is now [b]{theme}[/].", title="Theme updated", timeout=2.5)


app = Posting()
if __name__ == "__main__":
    app.run()
