import os
from pathlib import Path
from typing import Any, Literal
from dotenv import dotenv_values
import httpx
from rich.console import Group
from rich.text import Text
from textual import on, log, work
from textual.command import CommandPalette
from textual.css.query import NoMatches
from textual.design import ColorSystem
from textual.events import Click
from textual.reactive import Reactive, reactive
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.screen import Screen
from textual.signal import Signal
from textual.widget import Widget
from textual.widgets import (
    Button,
    Footer,
    Input,
    Label,
    TextArea,
)
from textual.widgets._tabbed_content import ContentTab
from posting.collection import (
    Collection,
    Cookie,
    HttpRequestMethod,
    Options,
    RequestModel,
)

from posting.commands import PostingProvider
from posting.config import SETTINGS, Settings
from posting.jump_overlay import JumpOverlay
from posting.jumper import Jumper
from posting.types import PostingLayout
from posting.user_host import get_user_host_string
from posting.variables import SubstitutionError
from posting.version import VERSION
from posting.widgets.collection.browser import (
    CollectionBrowser,
    CollectionTree,
)
from posting.widgets.datatable import PostingDataTable
from posting.widgets.request.header_editor import HeadersTable
from posting.messages import HttpResponseReceived
from posting.widgets.request.method_selection import (
    MethodSelectionPopup,
    MethodSelection,
)

from posting.widgets.request.query_editor import ParamsTable
from posting.widgets.request.request_auth import RequestAuth

from posting.widgets.request.request_body import RequestBodyTextArea
from posting.widgets.request.request_editor import RequestEditor
from posting.widgets.request.request_metadata import RequestMetadata
from posting.widgets.request.request_options import RequestOptions
from posting.widgets.request.url_bar import UrlInput, UrlBar
from posting.widgets.response.response_area import ResponseArea
from posting.widgets.response.response_trace import Event, ResponseTrace


class AppHeader(Horizontal):
    """The header of the app."""

    DEFAULT_CSS = """\
    AppHeader {
        color: $accent-lighten-2;
        padding: 0 3;
        margin-top: 1;
        height: 1;

        & > #app-title {
            dock: left;
        }

        & > #app-user-host {
            dock: right;
            color: $text-muted;
        }
    }
    """

    def compose(self) -> ComposeResult:
        settings = SETTINGS.get().heading
        yield Label(f"Posting [dim]{VERSION}[/]", id="app-title")
        if settings.show_host:
            yield Label(get_user_host_string(), id="app-user-host")

        self.set_class(not settings.visible, "hidden")


class AppBody(Vertical):
    """The body of the app."""

    DEFAULT_CSS = """\
    AppBody {
        padding: 0 2;
    }
    """


class MainScreen(Screen[None]):
    BINDINGS = [
        Binding("ctrl+j", "send_request", "Send"),
        Binding("ctrl+t", "change_method", "Method"),
        Binding("ctrl+l", "app.focus('url-input')", "Focus URL input", show=False),
        Binding("ctrl+s", "save_request", "Save"),
        Binding("ctrl+n", "new_request", "New"),
        Binding("ctrl+m", "toggle_maximized", "Expand", show=False),
    ]

    selected_method: Reactive[HttpRequestMethod] = reactive("GET", init=False)
    """The currently selected method of the request."""
    layout: Reactive[PostingLayout] = reactive("vertical", init=False)
    """The current layout of the app."""
    maximized: Reactive[Literal["request", "response"] | None] = reactive(
        None, init=False
    )
    """The currently maximized section of the main screen."""

    def __init__(
        self,
        collection: Collection,
        layout: PostingLayout,
        environment_files: tuple[Path, ...],
    ) -> None:
        super().__init__()
        self.collection = collection
        self.cookies: httpx.Cookies = httpx.Cookies()
        self._initial_layout: PostingLayout = layout
        self.environment_files = environment_files
        self.settings = SETTINGS.get()

    def on_mount(self) -> None:
        self.layout = self._initial_layout

    def compose(self) -> ComposeResult:
        yield AppHeader()
        yield UrlBar()
        with AppBody():
            yield CollectionBrowser(collection=self.collection)
            yield RequestEditor()
            yield ResponseArea()
        yield Footer()

    async def send_request(self) -> None:
        self.url_bar.clear_events()
        request_options = self.request_options.to_model()
        verify_ssl = request_options.verify_ssl
        proxy_url = request_options.proxy_url or None
        timeout = request_options.timeout
        auth = self.request_auth.to_httpx_auth()
        try:
            async with httpx.AsyncClient(
                verify=verify_ssl,
                proxy=proxy_url,
                timeout=timeout,
                auth=auth,
            ) as client:
                request = self.build_httpx_request(
                    request_options, client, apply_template=True
                )
                request.headers["User-Agent"] = (
                    f"Posting/{VERSION} (Terminal-based API client)"
                )
                print("-- sending request --")
                print(request)
                print(request.headers)
                print("follow redirects =", request_options.follow_redirects)
                print("verify =", request_options.verify_ssl)
                print("attach cookies =", request_options.attach_cookies)
                print("proxy =", proxy_url)
                print("timeout =", timeout)
                print("auth =", auth)
                response = await client.send(
                    request=request,
                    follow_redirects=request_options.follow_redirects,
                )
                print("response cookies =", response.cookies)
                self.post_message(HttpResponseReceived(response))
        except httpx.ConnectTimeout as connect_timeout:
            log.error("Connect timeout", connect_timeout)
            self.notify(
                severity="error",
                title="Connect timeout",
                message=f"Couldn't connect within {timeout} seconds.",
            )
        except Exception as e:
            log.error("Error sending request", e)
            log.error("Type of error", type(e))
            self.url_input.add_class("error")
            self.url_input.focus()
            self.notify(
                severity="error",
                title="Couldn't send request",
                message=str(e),
            )
        else:
            self.url_input.remove_class("error")

    @work(exclusive=True)
    async def send_via_worker(self) -> None:
        await self.send_request()

    @on(Button.Pressed, selector="SendRequestButton")
    @on(Input.Submitted, selector="UrlInput")
    def handle_submit_via_event(self) -> None:
        """Send the request."""
        self.send_via_worker()

    @on(HttpResponseReceived)
    def on_response_received(self, event: HttpResponseReceived) -> None:
        """Update the response area with the response."""
        self.response_area.response = event.response
        self.cookies.update(event.response.cookies)
        self.response_trace.trace_complete()

    @on(CollectionTree.RequestSelected)
    def on_request_selected(self, event: CollectionTree.RequestSelected) -> None:
        """Load a request model into the UI when a request is selected."""
        self.load_request_model(event.request)

    @on(CollectionTree.RequestCacheUpdated)
    def on_request_cache_updated(
        self, event: CollectionTree.RequestCacheUpdated
    ) -> None:
        """Update the autocomplete suggestions when the request cache is updated."""
        print(event.cached_base_urls)
        self.url_bar.cached_base_urls = event.cached_base_urls

    async def action_send_request(self) -> None:
        """Send the request."""
        self.send_via_worker()

    def action_change_method(self) -> None:
        """Change the method of the request."""
        self.method_selection()

    def action_toggle_maximized(self) -> None:
        """Toggle the maximized state of the app."""
        if self.maximized in {"request", "response"}:
            self.maximize_section(None)
        elif self.focused:
            # Maximize the currently focused section.
            if self.focus_within_request():
                self.maximize_section("request")
            elif self.focus_within_response():
                self.maximize_section("response")

    def maximize_section(self, section: Literal["request", "response"] | None) -> None:
        self.maximized = section

    def focus_within_request(self, focused: Widget | None = None) -> bool:
        """Whether the focus is within the request editor."""
        if focused is None:
            focused = self.focused
        if focused is None:
            return False
        return self.request_editor in focused.ancestors_with_self

    def focus_within_response(self, focused: Widget | None = None) -> bool:
        """Whether the focus is within the response area."""
        if focused is None:
            focused = self.focused
        if focused is None:
            return False
        return self.response_area in focused.ancestors_with_self

    async def action_save_request(self) -> None:
        """Save the request to disk, possibly prompting the user for more information
        if it's the first time this request has been saved to disk."""
        if self.collection_tree.currently_open is None:
            # No request currently open in the collection tree, we're saving a
            # request which the user may already have filled in some data of in
            # the UI.
            request_model = self.build_request_model(self.request_options.to_model())
            print("initial_request", request_model)
            await self.collection_tree.new_request_flow(request_model)
            # The new request flow is already handling the saving of the request to disk.
            # No further action is required.
            return

        # In this case, we're saving an existing request to disk.
        request_model = self.build_request_model(self.request_options.to_model())
        assert isinstance(
            request_model, RequestModel
        ), "currently open node should contain a request model"

        # At this point, either we're reusing the pre-existing home for the request
        # on disk, or the new location on disk which was assigned during the "new request flow"
        save_path = request_model.path
        if save_path is not None:
            request_model.save_to_disk(save_path)
            self.collection_browser.update_currently_open_node(request_model)
            self.notify(
                title="Request saved",
                message=f"{save_path.absolute().relative_to(Path.cwd())}",
                timeout=3,
            )

    async def action_new_request(self) -> None:
        """Open the new request flow."""
        await self.collection_tree.new_request_flow(None)

    def watch_layout(self, layout: Literal["horizontal", "vertical"]) -> None:
        """Update the layout of the app to be horizontal or vertical."""
        classes = {"horizontal", "vertical"}
        other_class = classes.difference({layout}).pop()
        self.app_body.add_class(f"layout-{layout}")
        self.app_body.remove_class(f"layout-{other_class}")

    def watch_maximized(self, maximized: Literal["request", "response"] | None) -> None:
        """Hide the non-maximized section."""
        request_editor = self.request_editor
        response_area = self.response_area

        # Hide the request editor if the response area is currently maximized,
        # and vice-versa.
        request_editor.set_class(maximized == "response", "hidden")
        response_area.set_class(maximized == "request", "hidden")

    @on(TextArea.Changed, selector="RequestBodyTextArea")
    def on_request_body_change(self, event: TextArea.Changed) -> None:
        """Update the body tab to indicate if there is a body."""
        body_tab = self.query_one("#--content-tab-body-pane", ContentTab)
        if event.text_area.text:
            body_tab.update("Body[cyan b]•[/]")
        else:
            body_tab.update("Body")

    @on(PostingDataTable.RowsRemoved, selector="HeadersTable")
    @on(PostingDataTable.RowsAdded, selector="HeadersTable")
    def on_content_changed(
        self, event: PostingDataTable.RowsRemoved | PostingDataTable.RowsAdded
    ) -> None:
        """Update the headers tab to indicate if there are any headers."""
        headers_tab = self.query_one("#--content-tab-headers-pane", ContentTab)
        if event.data_table.row_count:
            headers_tab.update("Headers[cyan b]•[/]")
        else:
            headers_tab.update("Headers")

    @on(PostingDataTable.RowsRemoved, selector="ParamsTable")
    @on(PostingDataTable.RowsAdded, selector="ParamsTable")
    def on_params_changed(
        self, event: PostingDataTable.RowsRemoved | PostingDataTable.RowsAdded
    ) -> None:
        """Update the parameters tab to indicate if there are any parameters."""
        params_tab = self.query_one("#--content-tab-parameters-pane", ContentTab)
        if event.data_table.row_count:
            params_tab.update("Parameters[cyan b]•[/]")
        else:
            params_tab.update("Parameters")

    @on(MethodSelection.Clicked)
    def method_selection(self) -> None:
        """Open a popup to select the method."""

        def set_method(method: str) -> None:
            self.selected_method = method

        self.app.push_screen(MethodSelectionPopup(), callback=set_method)

    def build_httpx_request(
        self,
        request_options: Options,
        client: httpx.AsyncClient,
        apply_template: bool,
    ) -> httpx.Request:
        """Build an httpx request from the UI."""
        request_model = self.build_request_model(request_options)
        if apply_template:
            variables = {
                f"env:{key}": value
                for file in self.environment_files
                for key, value in dotenv_values(file).items()
            }
            if self.settings.use_host_environment:
                host_env_variables = {
                    f"env:{key}": value for key, value in os.environ.items()
                }
                variables = {**variables, **host_env_variables}

            try:
                request_model.apply_template(variables)
            except SubstitutionError as e:
                log.error(e)
                raise
        request = request_model.to_httpx(client)
        request.extensions = {"trace": self.log_request_trace_event}
        return request

    async def log_request_trace_event(self, event: Event, info: dict[str, Any]) -> None:
        """Log an event to the request trace."""
        await self.response_trace.log_event(event, info)
        self.url_bar.log_event(event, info)

    def build_request_model(self, request_options: Options) -> RequestModel:
        """Grab data from the UI and pull it into a request model. This model
        may be passed around, stored on disk, etc."""
        open_node = self.collection_tree.currently_open
        open_request = open_node.data if open_node else None

        # We ensure elsewhere that the we can only "open" requests, not collection nodes.
        assert not isinstance(open_request, Collection)

        headers = self.headers_table.to_model()
        return RequestModel(
            name=self.request_metadata.request_name,
            path=open_request.path if open_request else None,
            description=self.request_metadata.description,
            method=self.selected_method,
            url=self.url_input.value.strip(),
            params=self.params_table.to_model(),
            headers=headers,
            options=request_options,
            cookies=(
                Cookie.from_httpx(self.cookies)
                if request_options.attach_cookies
                else []
            ),
            **self.request_editor.to_request_model_args(),
        )

    def load_request_model(self, request_model: RequestModel) -> None:
        """Load a request model into the UI."""
        self.selected_method = request_model.method
        self.url_input.value = str(request_model.url)
        self.params_table.replace_all_rows(
            [(param.name, param.value) for param in request_model.params]
        )
        self.headers_table.replace_all_rows(
            [(header.name, header.value) for header in request_model.headers]
        )
        if request_model.body:
            if request_model.body.content:
                # Set the body content in the text area and ensure the content
                # switcher is set such that the text area is visible.
                self.request_body_text_area.text = request_model.body.content
                self.request_editor.request_body_type_select.value = "text-body-editor"
                self.request_editor.form_editor.replace_all_rows([])
            elif request_model.body.form_data:
                self.request_editor.form_editor.replace_all_rows(
                    (param.name, param.value) for param in request_model.body.form_data
                )
                self.request_editor.request_body_type_select.value = "form-body-editor"
                self.request_body_text_area.text = ""
        else:
            self.request_body_text_area.text = ""
            self.request_editor.form_editor.replace_all_rows([])
            self.request_editor.request_body_type_select.value = "no-body-label"

        self.request_metadata.request = request_model
        self.request_options.load_options(request_model.options)
        self.request_auth.load_auth(request_model.auth)

    @property
    def url_bar(self) -> UrlBar:
        return self.query_one(UrlBar)

    @property
    def url_input(self) -> UrlInput:
        return self.query_one(UrlInput)

    @property
    def request_editor(self) -> RequestEditor:
        return self.query_one(RequestEditor)

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

    @property
    def request_metadata(self) -> RequestMetadata:
        return self.query_one(RequestMetadata)

    @property
    def collection_browser(self) -> CollectionBrowser:
        return self.query_one(CollectionBrowser)

    @property
    def request_auth(self) -> RequestAuth:
        return self.query_one(RequestAuth)

    @property
    def collection_tree(self) -> CollectionTree:
        return self.query_one(CollectionTree)

    @property
    def response_trace(self) -> ResponseTrace:
        return self.query_one(ResponseTrace)

    def watch_selected_method(self, value: str) -> None:
        self.query_one(MethodSelection).set_method(value)


class PostingApp(App[None]):
    def __init__(
        self,
        settings: Settings,
        environment_files: tuple[Path, ...],
        collection: Collection,
        collection_specified: bool = False,
    ) -> None:
        super().__init__()

        SETTINGS.set(settings)

        self.settings = settings
        self.environment_files = environment_files
        self.collection = collection
        self.collection_specified = collection_specified

        self.animation_level = settings.animation


class Posting(PostingApp):
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
        "posting": ColorSystem(
            primary="#004578",
            secondary="#0178D4",
            warning="#ffa62b",
            error="#ba3c5b",
            success="#4EBF71",
            accent="#ffa62b",
            dark=True,
        ),
        "monokai": ColorSystem(
            primary="#F92672",  # Pink
            secondary="#66D9EF",  # Light Blue
            warning="#FD971F",  # Orange
            error="#F92672",  # Pink (same as primary for consistency)
            success="#A6E22E",  # Green
            accent="#AE81FF",  # Purple
            background="#272822",  # Dark gray-green
            surface="#3E3D32",  # Slightly lighter gray-green
            panel="#3E3D32",  # Same as surface for consistency
            dark=True,
        ),
        "solarized-light": ColorSystem(
            primary="#268bd2",
            secondary="#2aa198",
            warning="#cb4b16",
            error="#dc322f",
            success="#859900",
            accent="#6c71c4",
            background="#fdf6e3",
            surface="#eee8d5",
            panel="#eee8d5",
        ),
        "nautilus": ColorSystem(
            primary="#0077BE",  # Ocean Blue
            secondary="#20B2AA",  # Light Sea Green
            warning="#FFD700",  # Gold (like sunlight on water)
            error="#FF6347",  # Tomato (like a warning buoy)
            success="#32CD32",  # Lime Green (like seaweed)
            accent="#FF8C00",  # Dark Orange (like a sunset over water)
            dark=True,
            background="#001F3F",  # Dark Blue (deep ocean)
            surface="#003366",  # Navy Blue (shallower water)
            panel="#005A8C",  # Steel Blue (water surface)
        ),
        "galaxy": ColorSystem(
            primary="#8A2BE2",  # Improved Deep Magenta (Blueviolet)
            secondary="#9370DB",  # Softer Dusky Indigo (Medium Purple)
            warning="#FFD700",  # Gold, more visible than orange
            error="#FF4500",  # OrangeRed, vibrant but less harsh than pure red
            success="#00FA9A",  # Medium Spring Green, kept for vibrancy
            accent="#FF69B4",  # Hot Pink, for a pop of color
            dark=True,
            background="#0F0F1F",  # Very Dark Blue, almost black
            surface="#1E1E3F",  # Dark Blue-Purple
            panel="#2D2B55",  # Slightly Lighter Blue-Purple
        ),
        "nebula": ColorSystem(
            primary="#4169E1",  # Royal Blue, more vibrant than Midnight Blue
            secondary="#9400D3",  # Dark Violet, more vibrant than Indigo Dye
            warning="#FFD700",  # Kept Gold for warnings
            error="#FF1493",  # Deep Pink, more nebula-like than Crimson
            success="#00FF7F",  # Spring Green, slightly more vibrant
            accent="#FF00FF",  # Magenta, for a true neon accent
            dark=True,
            background="#0A0A23",  # Dark Navy, closer to a night sky
            surface="#1C1C3C",  # Dark Blue-Purple
            panel="#2E2E5E",  # Slightly Lighter Blue-Purple
        ),
        "alpine": ColorSystem(
            primary="#4A90E2",  # Clear Sky Blue
            secondary="#81A1C1",  # Misty Blue
            warning="#EBCB8B",  # Soft Sunlight
            error="#BF616A",  # Muted Red
            success="#A3BE8C",  # Alpine Meadow Green
            accent="#5E81AC",  # Mountain Lake Blue
            dark=True,
            background="#2E3440",  # Dark Slate Grey
            surface="#3B4252",  # Darker Blue-Grey
            panel="#434C5E",  # Lighter Blue-Grey
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
        "twilight": ColorSystem(
            primary="#367588",
            secondary="#5F9EA0",
            warning="#FFD700",
            error="#FF6347",
            success="#00FA9A",
            accent="#FF7F50",
            dark=True,
            background="#191970",
            surface="#3B3B6D",
            panel="#4C516D",
        ),
        "hacker": ColorSystem(
            primary="#00FF00",  # Bright Green (Lime)
            secondary="#32CD32",  # Lime Green
            warning="#ADFF2F",  # Green Yellow
            error="#FF4500",  # Orange Red (for contrast)
            success="#00FA9A",  # Medium Spring Green
            accent="#39FF14",  # Neon Green
            dark=True,
            background="#0D0D0D",  # Almost Black
            surface="#1A1A1A",  # Very Dark Gray
            panel="#2A2A2A",  # Dark Gray
        ),
    }

    theme: Reactive[str | None] = reactive("posting", init=False)
    _jumping: Reactive[bool] = reactive(False, init=False, bindings=True)

    def on_mount(self) -> None:
        self.jumper = Jumper(
            {
                "collection-tree": "tab",
                "--content-tab-headers-pane": "q",
                "--content-tab-body-pane": "w",
                "--content-tab-parameters-pane": "e",
                "--content-tab-auth-pane": "r",
                "--content-tab-metadata-pane": "t",
                "--content-tab-options-pane": "y",
                "--content-tab-response-body-pane": "a",
                "--content-tab-response-headers-pane": "s",
                "--content-tab-response-cookies-pane": "d",
                "--content-tab-response-trace-pane": "f",
            },
            screen=self.screen,
        )
        self.theme_change_signal = Signal[ColorSystem](self, "theme-changed")
        self.theme = self.settings.theme

    def get_default_screen(self) -> MainScreen:
        self.main_screen = MainScreen(
            collection=self.collection,
            layout=self.settings.layout,
            environment_files=self.environment_files,
        )
        if not self.collection_specified:
            self.notify(
                "Using the default collection directory.",
                title="No collection specified",
                severity="warning",
                timeout=7,
            )
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
        self.notify(
            f"Theme is now [b]{theme!r}[/].", title="Theme updated", timeout=2.5
        )

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

    def watch_theme(self, theme: str | None) -> None:
        self.refresh_css(animate=False)
        self.screen._update_styles()
        if theme:
            self.theme_change_signal.publish(self.themes[theme])

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
