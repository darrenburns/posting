from importlib.metadata import version
from pathlib import Path
from typing import Literal
import httpx
from rich.console import Group
from rich.text import Text
from textual import on, log
from textual.command import CommandPalette
from textual.css.query import NoMatches
from textual.design import ColorSystem
from textual.events import Click
from textual.reactive import Reactive, reactive
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.screen import Screen
from textual.widget import Widget
from textual.widgets import (
    Button,
    Footer,
    Input,
    Label,
    TextArea,
)
from textual.widgets._tabbed_content import ContentTab

from posting.commands import PostingProvider
from posting.jump_overlay import JumpOverlay
from posting.jumper import Jumper
from posting.widgets.datatable import PostingDataTable
from posting.widgets.request.header_editor import HeadersTable
from posting.messages import HttpResponseReceived
from posting.widgets.request.method_selection import (
    MethodSelectionPopup,
    MethodSelection,
)

from posting.widgets.request.query_editor import ParamsTable

from posting.widgets.request.request_body import RequestBodyTextArea
from posting.widgets.request.request_editor import RequestEditor
from posting.widgets.request.request_options import RequestOptions
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
        Binding("ctrl+l", "app.focus('url-input')", "Focus URL input", show=False),
        # Binding("ctrl+n", "tree", "DEBUG Show tree"),
    ]

    selected_method = reactive("GET", init=False)
    layout: Reactive[Literal["horizontal", "vertical"]] = reactive("vertical")

    def __init__(self) -> None:
        super().__init__()
        self.cookies = httpx.Cookies()

    def compose(self) -> ComposeResult:
        yield AppHeader(f"Posting [white dim]{version('posting')}[/]")
        yield UrlBar()
        with AppBody():
            yield RequestEditor()
            yield ResponseArea()
        yield Footer()

    @on(Button.Pressed, selector="SendRequestButton")
    @on(Input.Submitted, selector="UrlInput")
    async def send_request(self) -> None:
        request_options = self.request_options
        try:
            async with httpx.AsyncClient(verify=request_options.verify) as client:
                request = self.build_httpx_request(request_options)
                print("-- sending request --")
                print(request)
                print(request.headers)
                print("follow redirects =", request_options.follow_redirects)
                print("verify =", request_options.verify)
                print("attach cookies =", request_options.attach_cookies)
                response = await client.send(
                    request=request,
                    follow_redirects=request_options.follow_redirects,
                )
                self.post_message(HttpResponseReceived(response))

        except Exception as e:
            log.error("Error sending request", e)
            self.url_input.add_class("error")
            self.url_input.focus()
            self.notify(
                severity="error",
                title="Couldn't send request",
                message=str(e),
            )
        else:
            self.url_input.remove_class("error")

    @on(HttpResponseReceived)
    def on_response_received(self, event: HttpResponseReceived) -> None:
        self.response_area.response = event.response
        self.cookies.update(event.response.cookies)

    async def action_send_request(self) -> None:
        await self.send_request()

    def action_change_method(self) -> None:
        self.method_selection()

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

    @on(PostingDataTable.Changed, selector="HeadersTable")
    def on_content_changed(self, event: PostingDataTable.Changed) -> None:
        print("on_content_changed")
        headers_tab = self.query_one("#--content-tab-headers-pane", ContentTab)
        print("event.data_table.row_count", event.data_table.row_count)
        if event.data_table.row_count:
            headers_tab.update("Headers[cyan b]•[/]")
        else:
            headers_tab.update("Headers")

    @on(PostingDataTable.Changed, selector="ParamsTable")
    def on_params_changed(self, event: PostingDataTable.Changed) -> None:
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

    def build_httpx_request(self, request_options: RequestOptions) -> httpx.Request:
        return httpx.Request(
            method=self.selected_method,
            url=self.url_input.value.strip(),
            params=self.params_table.as_dict(),
            content=self.request_body_text_area.text,
            headers=self.headers_table.as_dict(),
            cookies=self.cookies if request_options.attach_cookies else None,
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

    @property
    def request_options(self) -> RequestOptions:
        return self.query_one(RequestOptions)

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
        Binding(
            "ctrl+o",
            "toggle_jump_mode",
            description="Jump",
            show=True,
        ),
    ]

    themes: dict[str, ColorSystem] = {
        "textual": ColorSystem(
            primary="#004578",
            secondary="#0178D4",
            warning="#ffa62b",
            error="#ba3c5b",
            success="#4EBF71",
            accent="#ffa62b",
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
        "ocean-light": ColorSystem(
            primary="#1e90ff",
            secondary="#00ced1",
            warning="#ffa07a",
            error="#ff4500",
            success="#20b2aa",
            accent="#4682b4",
            background="#f0f8ff",
            surface="#f0f8ff",
            panel="#f0f8ff",
        ),
        "forest": ColorSystem(
            primary="#006B3F",  # Deep Forest Green
            secondary="#8A9A5B",  # Moss Green
            warning="#DAA520",  # Goldenrod
            error="#8B0000",  # Dark Red
            success="#228B22",  # Dark Forest Green
            accent="#8FBC8B",  # Dusty Sea Green
            dark=True,
        ),
        "galaxy": ColorSystem(
            primary="#571089",  # Deep Magenta
            secondary="#603ca6",  # Dusky Indigo
            warning="#ff9900",  # Vivid Orange for warnings
            error="#d00000",  # Vivid Red for errors
            success="#4cc9f0",  # Bright Cyan for success
            accent="#bc6ff1",  # Bright Lilac
            dark=True,  # Emphasizing a dark theme
            surface="#32174d",  # Dark Purple
            panel="#452864",  # Slightly Lighter Dark Purple
        ),
        "nebula": ColorSystem(
            primary="#191970",  # Midnight Blue
            secondary="#4B0082",  # Indigo Dye
            warning="#FFD700",  # Gold, for a visually distinct warning
            error="#DC143C",  # Crimson, for a striking error indication
            success="#00FA9A",  # Medium Spring Green, for a refreshing success visualization
            accent="#FF6FFF",  # Neon Pink-Purple
            dark=True,  # Dedicated to a dark theme aesthetic
            surface="#242124",  # Raisin Black
            panel="#313131",  # Dark Charcoal
            background="#1B1B1B",  # Eerie Black
        ),
        "cobalt": ColorSystem(
            primary="#334D5C",  # Deep Cobalt Blue
            secondary="#4878A6",  # Slate Blue
            warning="#FFAA22",  # Amber, suitable for warnings related to primary
            error="#E63946",  # Red, universally recognized for errors
            success="#4CAF50",  # Green, commonly used for success indication
            accent="#D94E64",  # Candy Apple Red
            dark=True,
            surface="#27343B",  # Dark Lead
            panel="#2D3E46",  # Storm Gray
            background="#1F262A",  # Charcoal
        ),
        "royal": ColorSystem(
            primary="#483D8B",  # Dark Slate Blue, a deep and rich primary color evoking a sense of royalty and depth
            secondary="#6A5ACD",  # Slate Blue, slightly lighter than the primary, but maintains the regal theme
            warning="#FFD700",  # Gold, for striking, majestic warnings that draw attention
            error="#B22222",  # Firebrick, a strong and important red for errors, ensuring they are noticed
            success="#228B22",  # Forest Green, an earthy, rich color for success, giving a subtle nod to traditional regal gardens
            accent="#9370DB",  # Medium Purple, a regal accent that stands out well against the darker shades
            dark=True,  # Emphasizing a dark, sophisticated theme
            surface="#39324B",  # A slightly muted version of Dark Slate Blue enhancing UI depth
            panel="#504A65",  # A medium dark blue-purple to provide subtle contrast within UI elements
            background="#2E2E40",  # A very deep blue tinged with purple, providing a solemn royal backdrop
        ),
        "twilight": ColorSystem(
            primary="#367588",  # Teal Blue
            secondary="#5F9EA0",  # Cadet Blue
            warning="#FFD700",  # Gold, for a noticeable yet elegant warning
            error="#CD5C5C",  # Indian Red, for urgent yet harmonious alerts
            success="#32CD32",  # Lime Green, a fresh and positive success indicator
            accent="#FF7F50",  # Coral
            dark=True,  # Emphasizing the low-light conditions
            surface="#480ca8",
            panel="#4C516D",  # Space Cadet
            background="#191970",  # Midnight Blue
        ),
    }

    theme: Reactive[str | None] = reactive("textual", init=False)
    _jumping: Reactive[bool] = reactive(False, init=False, bindings=True)

    def on_mount(self) -> None:
        self.jumper = Jumper(
            {
                "--content-tab-headers-pane": "q",
                "--content-tab-body-pane": "w",
                "--content-tab-parameters-pane": "e",
                "--content-tab-options-pane": "r",
                "--content-tab-response-body-pane": "a",
                "--content-tab-response-headers-pane": "s",
                "--content-tab-response-cookies-pane": "d",
            },
            screen=self.screen,
        )

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

    @on(CommandPalette.Opened)
    def palette_opened(self) -> None:
        # Record the theme being used before the palette is opened.
        self._original_theme = self.theme

    @on(CommandPalette.OptionHighlighted)
    def palette_option_highlighted(
        self, event: CommandPalette.OptionHighlighted
    ) -> None:
        prompt: Group = event.highlighted_event.option.prompt
        # TODO: This is making quite a lot of assumptions. Fragile, but the only
        # way I can think of doing it given the current Textual APIs.
        command_name = prompt.renderables[0]
        if isinstance(command_name, Text):
            command_name = command_name.plain
        command_name = command_name.strip()
        if ":" in command_name:
            name, value = command_name.split(":", maxsplit=1)
            name = name.strip()
            value = value.strip()
            if name == "theme":
                if value in self.themes:
                    self.theme = value
            else:
                self.theme = self._original_theme

    @on(CommandPalette.Closed)
    def palette_closed(self, event: CommandPalette.Closed) -> None:
        # If we closed with a result, that will be handled by the command
        # being triggered. However, if we closed the palette with no result
        # then make sure we revert the theme back.
        if not event.option_selected:
            self.theme = self._original_theme

    def watch_theme(self) -> None:
        self.refresh_css(animate=False)
        self.screen._update_styles()

    def action_toggle_jump_mode(self) -> None:
        self._jumping = not self._jumping

    async def watch__jumping(self, jumping: bool) -> None:
        def handle_jump_target(target: str | Widget | None) -> None:
            if isinstance(target, str):
                try:
                    target_widget = self.screen.query_one(f"#{target}")
                except NoMatches:
                    log.warning(
                        f"Attempted to jump to target #{target}, but it couldn't be found on {self.screen!r}"
                    )
                else:
                    if target_widget.focusable:
                        target_widget.focus()
                    else:
                        target_widget.post_message(
                            Click(0, 0, 0, 0, 0, False, False, False)
                        )

            elif isinstance(target, Widget):
                target.focus()

        self.clear_notifications()
        await self.push_screen(JumpOverlay(self.jumper), callback=handle_jump_target)


app = Posting()
if __name__ == "__main__":
    app.run()
