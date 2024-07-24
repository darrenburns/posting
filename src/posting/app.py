from pathlib import Path
from typing import Any, Literal, Union, cast
import subprocess
import itertools
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
from posting.help_screen import HelpScreen
from posting.jump_overlay import JumpOverlay
from posting.jumper import Jumper
from posting.themes import BUILTIN_THEMES, Theme, load_user_themes
from posting.types import CertTypes, PostingLayout
from posting.user_host import get_user_host_string
from posting.variables import SubstitutionError, get_variables
from posting.version import VERSION
from posting.widgets.collection.browser import (
    CollectionBrowser,
    CollectionTree,
)
from posting.widgets.datatable import PostingDataTable
from posting.variables import load_variables
from posting.widgets.request.header_editor import HeadersTable
from posting.messages import HttpResponseReceived
from posting.widgets.request.method_selection import MethodSelector

from posting.widgets.request.query_editor import ParamsTable
from posting.widgets.request.request_auth import RequestAuth

from posting.widgets.request.request_body import RequestBodyTextArea
from posting.widgets.request.request_editor import RequestEditor
from posting.widgets.request.request_metadata import RequestMetadata
from posting.widgets.request.request_options import RequestOptions
from posting.widgets.request.url_bar import UrlInput, UrlBar
from posting.widgets.response.response_area import ResponseArea
from posting.widgets.response.response_trace import Event, ResponseTrace
from posting.xresources import load_xresources_themes


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
        if settings.show_version:
            yield Label(f"Posting [dim]{VERSION}[/]", id="app-title")
        else:
            yield Label("Posting", id="app-title")
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
    AUTO_FOCUS = None
    BINDINGS = [
        Binding("ctrl+j", "send_request", "Send"),
        Binding("ctrl+t", "change_method", "Method"),
        Binding("ctrl+l", "app.focus('url-input')", "Focus URL input", show=False),
        Binding("ctrl+s", "save_request", "Save"),
        Binding("ctrl+n", "new_request", "New"),
        Binding("ctrl+m", "toggle_maximized", "Expand section", show=False),
        Binding(
            "ctrl+h",
            "toggle_collection_browser",
            "Toggle collection browser",
            show=False,
        ),
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
        load_variables(self.environment_files, self.settings.use_host_environment)

    def on_mount(self) -> None:
        self.layout = self._initial_layout

        # Set the initial focus based on the settings.
        focus_on_startup = self.settings.focus.on_startup
        if focus_on_startup == "url":
            target = self.url_bar.url_input
        elif focus_on_startup == "method":
            target = self.method_selector
        elif focus_on_startup == "collection":
            target = self.collection_browser.collection_tree
        else:
            target = None

        if target is not None:
            self.set_focus(target)

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

        cert_config = SETTINGS.get().ssl
        httpx_cert_config: list[str] = []
        if certificate_path := cert_config.certificate_path:
            httpx_cert_config.append(certificate_path)
        if key_file := cert_config.key_file:
            httpx_cert_config.append(key_file)
        if password := cert_config.password:
            httpx_cert_config.append(password.get_secret_value())

        verify: str | bool = verify_ssl
        if verify_ssl and cert_config.ca_bundle is not None:
            # If verification is enabled and a CA bundle is supplied,
            # use the CA bundle.
            verify = cert_config.ca_bundle

        cert = cast(CertTypes, tuple(httpx_cert_config))
        try:
            async with httpx.AsyncClient(
                verify=verify,
                cert=cert,
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

    @on(MethodSelector.MethodChanged)
    def on_method_selector_changed(self, event: MethodSelector.MethodChanged) -> None:
        self.selected_method = event.value

    @on(Button.Pressed, selector="SendRequestButton")
    @on(Input.Submitted, selector="UrlInput")
    def handle_submit_via_event(self) -> None:
        """Send the request."""
        self.send_via_worker()

    @on(HttpResponseReceived)
    def on_response_received(self, event: HttpResponseReceived) -> None:
        """Update the response area with the response."""

        # If the config to automatically move the focus on receipt
        # of a response has been set, move focus as required.
        focus_on_response = self.settings.focus.on_response
        if focus_on_response == "body":
            self.response_area.text_editor.text_area.focus()
        elif focus_on_response == "tabs":
            self.response_area.content_tabs.focus()

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
        self.url_bar.cached_base_urls = sorted(event.cached_base_urls)

    async def action_send_request(self) -> None:
        """Send the request."""
        self.send_via_worker()

    def action_change_method(self) -> None:
        """Change the method of the request."""
        method_selector = self.method_selector
        method_selector.focus()
        method_selector.expanded = not method_selector.expanded

    def action_toggle_collection_browser(self) -> None:
        """Toggle the collection browser."""
        collection_browser = self.collection_browser
        collection_browser.display = not collection_browser.display

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
            try:
                path_to_display = str(save_path.resolve().relative_to(Path.cwd()))
            except ValueError:
                path_to_display = save_path.name

            request_model.save_to_disk(save_path)
            self.collection_browser.update_currently_open_node(request_model)
            self.notify(
                title="Request saved",
                message=path_to_display,
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
        params_tab = self.query_one("#--content-tab-query-pane", ContentTab)
        if event.data_table.row_count:
            params_tab.update("Query[cyan b]•[/]")
        else:
            params_tab.update("Query")

    def build_httpx_request(
        self,
        request_options: Options,
        client: httpx.AsyncClient,
        apply_template: bool,
    ) -> httpx.Request:
        """Build an httpx request from the UI."""
        request_model = self.build_request_model(request_options)
        if apply_template:
            variables = get_variables()
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
            auth=self.request_auth.to_model(),
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
        self.method_selector.value = request_model.method
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
    def method_selector(self) -> MethodSelector:
        return self.query_one(MethodSelector)

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


class Posting(App[None]):
    COMMANDS = {PostingProvider}
    CSS_PATH = Path(__file__).parent / "posting.scss"
    BINDINGS = [
        Binding(
            "ctrl+p",
            "command_palette",
            description="Commands",
        ),
        Binding(
            "ctrl+o",
            "toggle_jump_mode",
            description="Jump",
        ),
        Binding("f1,ctrl+question_mark", "help", "Help"),
    ]

    def __init__(
        self,
        settings: Settings,
        environment_files: tuple[Path, ...],
        collection: Collection,
        collection_specified: bool = False,
    ) -> None:
        SETTINGS.set(settings)

        available_themes: dict[str, Theme] = {"posting": BUILTIN_THEMES["posting"]}

        if settings.load_builtin_themes:
            available_themes |= BUILTIN_THEMES

        if settings.use_xresources:
            available_themes |= load_xresources_themes()

        if settings.load_user_themes:
            available_themes |= load_user_themes()

        self.themes = available_themes

        # We need to call super.__init__ after the themes are loaded,
        # because our `get_css_variables` override depends on
        # the themes dict being available.
        super().__init__()

        self.settings = settings
        self.environment_files = environment_files
        self.collection = collection
        self.collection_specified = collection_specified
        self.animation_level = settings.animation

    theme: Reactive[str | None] = reactive("posting", init=False)
    _jumping: Reactive[bool] = reactive(False, init=False, bindings=True)

    def on_mount(self) -> None:
        self.jumper = Jumper(
            {
                "method-selector": "1",
                "url-input": "2",
                "collection-tree": "tab",
                "--content-tab-headers-pane": "q",
                "--content-tab-body-pane": "w",
                "--content-tab-query-pane": "e",
                "--content-tab-auth-pane": "r",
                "--content-tab-info-pane": "t",
                "--content-tab-options-pane": "y",
                "--content-tab-response-body-pane": "a",
                "--content-tab-response-headers-pane": "s",
                "--content-tab-response-cookies-pane": "d",
                "--content-tab-response-trace-pane": "f",
            },
            screen=self.screen,
        )
        self.theme_change_signal = Signal[Theme](self, "theme-changed")
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
            theme = self.themes.get(self.theme)
            if theme:
                color_system = theme.to_color_system().generate()
            else:
                color_system = {}
        else:
            color_system = {}
        return {**super().get_css_variables(), **color_system}

    def command_layout(self, layout: Literal["vertical", "horizontal"]) -> None:
        self.main_screen.layout = layout

    def command_theme(self, theme: str) -> None:
        self.theme = theme
        self.notify(
            f"Theme is now [b]{theme!r}[/].", title="Theme updated", timeout=2.5
        )

    @on(CommandPalette.Opened)
    def palette_opened(self) -> None:
        # If the theme preview is disabled, don't record the theme being used
        # before the palette is opened.
        if not self.settings.command_palette.theme_preview:
            return

        # Record the theme being used before the palette is opened.
        self._original_theme = self.theme

    @on(CommandPalette.OptionHighlighted)
    def palette_option_highlighted(
        self, event: CommandPalette.OptionHighlighted
    ) -> None:
        # If the theme preview is disabled, don't update the theme when an option
        # is highlighted.
        if not self.settings.command_palette.theme_preview:
            return

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
        if not self.settings.command_palette.theme_preview:
            return
        if not event.option_selected:
            self.theme = self._original_theme

    def watch_theme(self, theme: str | None) -> None:
        self.refresh_css(animate=False)
        self.screen._update_styles()
        if theme:
            self.theme_change_signal.publish(self.themes[theme])

    def action_toggle_jump_mode(self) -> None:
        self._jumping = not self._jumping

    def watch__jumping(self, jumping: bool) -> None:
        focused_before = self.focused
        if focused_before is not None:
            self.set_focus(None, scroll_visible=False)

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
                        self.set_focus(target_widget)
                    else:
                        target_widget.post_message(
                            Click(0, 0, 0, 0, 0, False, False, False)
                        )

            elif isinstance(target, Widget):
                self.set_focus(target)
            else:
                # If there's no target (i.e. the user pressed ESC to dismiss)
                # then re-focus the widget that was focused before we opened
                # the jumper.
                if focused_before is not None:
                    self.set_focus(focused_before, scroll_visible=False)

        self.clear_notifications()
        self.push_screen(JumpOverlay(self.jumper), callback=handle_jump_target)

    async def action_help(self) -> None:
        focused = self.focused

        def reset_focus(_) -> None:
            if focused:
                self.screen.set_focus(focused)

        self.set_focus(None)
        await self.push_screen(HelpScreen(widget=focused), callback=reset_focus)
