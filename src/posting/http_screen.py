import asyncio
import inspect
from contextlib import redirect_stdout, redirect_stderr
from pathlib import Path
from typing import Any, Literal, cast

from aiohttp import ClientWebSocketResponse
import aiohttp
import httpx
from textual.css.query import NoMatches
from textual.events import Click

from posting.importing.curl import CurlImport
from textual import on, log, work
from textual.reactive import Reactive, reactive
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Container, Horizontal, Vertical
from textual.screen import Screen
from textual.widget import Widget
from textual.widgets import (
    Button,
    ContentSwitcher,
    Footer,
    Input,
    Label,
    TextArea,
)
from textual.widgets._tabbed_content import ContentTab
from posting.collection import (
    Collection,
    Cookie,
    Header,
    RequestType,
    Options,
    HttpRequestModel,
    WebsocketRequestModel,
)

from posting.config import SETTINGS
from posting.jump_overlay import JumpOverlay
from posting.jumper import Jumper
from posting.scripts import execute_script, Posting as PostingContext
from posting.types import CertTypes, PostingLayout
from posting.user_host import get_user_host_string
from posting.variables import SubstitutionError, get_variables
from posting.version import VERSION
from posting.widgets.collection.browser import (
    CollectionBrowser,
    CollectionTree,
)
from posting.widgets.datatable import PostingDataTable
from posting.widgets.request.header_editor import HeadersTable
from posting.messages import HttpResponseReceived
from posting.widgets.request.request_type_selection import RequestTypeSelector

from posting.widgets.request.query_editor import ParamsTable
from posting.widgets.request.request_auth import RequestAuth

from posting.widgets.request.request_body import RequestBodyTextArea
from posting.widgets.request.request_editor import RequestEditor
from posting.widgets.request.request_metadata import RequestMetadata
from posting.widgets.request.request_options import RequestOptions
from posting.widgets.request.request_scripts import RequestScripts
from posting.widgets.request.url_bar import CurlMessage, UrlInput, UrlBar
from posting.widgets.response.response_area import ResponseArea
from posting.widgets.response.response_trace import Event, ResponseTrace
from posting.widgets.response.script_output import ScriptOutput
from posting.widgets.rich_log import RichLogIO
from posting.widgets.websocket.replies import Replies
from posting.widgets.websocket.websocket_composer import (
    WebSocketConnected,
    WebSocketDisconnected,
    WebsocketComposer,
)
from posting.widgets.websocket.websocket_container import WebSocketContainer


class AppHeader(Horizontal):
    """The header of the app."""

    def compose(self) -> ComposeResult:
        settings = SETTINGS.get().heading
        if settings.show_version:
            yield Label(f"[b]Posting[/] [dim]{VERSION}[/]", id="app-title")
        else:
            yield Label("[b]Posting[/]", id="app-title")
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


class HttpScreen(Screen[None]):
    AUTO_FOCUS = None
    BINDING_GROUP_TITLE = "Main Screen"
    BINDINGS = [
        Binding(
            "ctrl+j,alt+enter",
            "send_request",
            "Send",
            tooltip="Send the current request.",
            id="send-request",
        ),
        Binding(
            "ctrl+t",
            "change_method",
            "Method",
            tooltip="Focus the method selector.",
            id="focus-method",
        ),
        Binding(
            "ctrl+l",
            "app.focus('url-input')",
            "Focus URL input",
            show=False,
            tooltip="Focus the URL input.",
            id="focus-url",
        ),
        Binding(
            "ctrl+s",
            "save_request",
            "Save",
            tooltip="Save the current request. If a request is open, this will overwrite it.",
            id="save-request",
        ),
        Binding(
            "ctrl+n",
            "new_request",
            "New",
            tooltip="Create a new request.",
            id="new-request",
        ),
        Binding(
            "ctrl+m",
            "toggle_expanded",
            "Expand section",
            show=False,
            tooltip="Expand or shrink the section (request or response) which has focus.",
            id="expand-section",
        ),
        Binding(
            "ctrl+o",
            "toggle_jump_mode",
            description="Jump",
            tooltip="Activate jump mode to quickly move focus between widgets.",
            id="jump",
        ),
        Binding(
            "ctrl+h",
            "toggle_collection_browser",
            "Toggle collection browser",
            show=False,
            tooltip="Toggle the collection browser.",
            id="toggle-collection",
        ),
    ]

    selected_request_type: Reactive[RequestType] = reactive("GET", init=False)
    """The currently selected method of the request."""
    current_layout: Reactive[PostingLayout] = reactive("vertical", init=False)
    """The current layout of the app."""
    expanded_section: Reactive[Literal["request", "response"] | None] = reactive(
        None, init=False
    )
    """The currently expanded section of the main screen."""

    _jumping: Reactive[bool] = reactive(False, init=False, bindings=True)
    """True if 'jump mode' is currently active, otherwise False."""

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
        self.websocket: ClientWebSocketResponse | None = None

    def on_mount(self) -> None:
        self.current_layout = self._initial_layout

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
                "--content-tab-scripts-pane": "y",
                "--content-tab-options-pane": "u",
                "--content-tab-response-body-pane": "a",
                "--content-tab-response-headers-pane": "s",
                "--content-tab-response-cookies-pane": "d",
                "--content-tab-response-scripts-pane": "f",
                "--content-tab-response-trace-pane": "g",
            },
            screen=self,
        )

        # Set the initial focus based on the settings.
        focus_on_startup = self.settings.focus.on_startup
        if focus_on_startup == "url":
            target = self.url_bar.url_input
        elif focus_on_startup == "method":
            target = self.request_type_selector
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
            collection_browser = CollectionBrowser(collection=self.collection)
            collection_browser.display = (
                self.settings.collection_browser.show_on_startup
            )
            yield collection_browser
            with ContentSwitcher(
                id="app-body-switcher", initial="app-body-http-container"
            ):
                with Container(id="app-body-http-container"):
                    yield RequestEditor(id="request-editor")
                    yield ResponseArea(id="response-area")
                yield WebSocketContainer(id="app-body-websocket-container")

        yield Footer(show_command_palette=False)

    def get_and_run_script(
        self,
        path_to_script: str,
        default_function_name: str,
        *args: Any,
    ) -> None:
        """
        Get and run a function from a script.

        Args:
            script_path: Path to the script, relative to the collection path.
            default_function_name: Default function name to use if not specified in the path.
            *args: Arguments to pass to the script function.
        """
        script_path = Path(path_to_script)
        path_name_parts = script_path.name.split(":")
        if len(path_name_parts) == 2:
            script_path = Path(path_name_parts[0])
            function_name = path_name_parts[1]
        else:
            function_name = default_function_name

        try:
            script_function = execute_script(
                self.collection.path, script_path, function_name
            )
        except Exception as e:
            log.error(f"Error loading script {function_name}: {e}")
            self.notify(
                severity="error",
                title=f"Error loading script {function_name}",
                message=f"The script at {script_path} could not be loaded: {e}",
            )
            raise

        if script_function is not None:
            try:
                script_output = self.response_script_output
                script_output.log_function_call_start(
                    f"{script_path.name}:{function_name}"
                )

                rich_log = script_output.rich_log
                stdout_log = RichLogIO(rich_log, "stdout")
                stderr_log = RichLogIO(rich_log, "stderr")

                with redirect_stdout(stdout_log), redirect_stderr(stderr_log):
                    # Ensure we pass in the number of parameters the user has
                    # implicitly requested in their script.
                    signature = inspect.signature(script_function)
                    num_params = len(signature.parameters)
                    if num_params > 0:
                        script_function(*args[:num_params])
                    else:
                        script_function()

                # Ensure any remaining content is flushed
                stdout_log.flush()
                stderr_log.flush()

            except Exception as e:
                log.error(f"Error running {function_name} script: {e}")
                self.notify(
                    severity="error",
                    title=f"Error running {function_name} script",
                    message=f"{e}",
                )
                raise
        else:
            log.warning(f"{function_name.capitalize()} script not found: {script_path}")
            self.notify(
                severity="error",
                title=f"{function_name.capitalize()} script not found",
                message=f"The {function_name} script at {script_path} could not be found.",
            )
            raise

    async def send_request(self) -> None:
        self.url_bar.clear_events()
        script_output = self.response_script_output
        script_output.reset()

        request_options = self.request_options.to_model()

        cert_config = SETTINGS.get().ssl
        httpx_cert_config: list[str] = []
        if certificate_path := cert_config.certificate_path:
            httpx_cert_config.append(certificate_path)
        if key_file := cert_config.key_file:
            httpx_cert_config.append(key_file)
        if password := cert_config.password:
            httpx_cert_config.append(password.get_secret_value())

        from posting.app import Posting

        app = cast("Posting", self.app)
        script_context = PostingContext(app)

        cert = cast(CertTypes, tuple(httpx_cert_config))
        try:
            # Run setup scripts first
            request_model = self.build_request_model(request_options)
            if setup_script := request_model.scripts.setup:
                try:
                    self.get_and_run_script(
                        setup_script,
                        "setup",
                        script_context,
                    )
                except Exception:
                    self.response_script_output.set_setup_status("error")
                else:
                    self.response_script_output.set_setup_status("success")
            else:
                self.response_script_output.set_setup_status("no-script")

            # Now apply the template
            variables = get_variables()
            try:
                request_model.apply_template(variables)
            except SubstitutionError as e:
                log.error(e)
                raise

            verify_ssl = request_model.options.verify_ssl
            verify: str | bool = verify_ssl
            if verify_ssl and cert_config.ca_bundle is not None:
                # If verification is enabled and a CA bundle is supplied,
                # use the CA bundle.
                verify = cert_config.ca_bundle

            timeout = request_model.options.timeout
            async with httpx.AsyncClient(
                verify=verify,
                cert=cert,
                proxy=request_model.options.proxy_url or None,
                timeout=timeout,
                auth=request_model.auth.to_httpx_auth() if request_model.auth else None,
            ) as client:
                script_context.request = request_model

                # If there's an associated pre-request script, run it.
                if on_request := request_model.scripts.on_request:
                    try:
                        self.get_and_run_script(
                            on_request,
                            "on_request",
                            request_model,
                            script_context,
                        )
                    except Exception:
                        self.response_script_output.set_request_status("error")
                        # TODO - load the error into the response area, or log it.
                    else:
                        self.response_script_output.set_request_status("success")
                else:
                    self.response_script_output.set_request_status("no-script")
                request = self.build_httpx_request(request_model, client)

                request.headers["User-Agent"] = (
                    f"Posting/{VERSION} (Terminal-based API client)"
                )
                print("-- sending request --")
                print(request)
                print(request.headers)
                print("follow redirects =", request_options.follow_redirects)
                print("verify =", request_options.verify_ssl)
                print("attach cookies =", request_options.attach_cookies)
                print("proxy =", request_model.options.proxy_url)
                print("timeout =", request_model.options.timeout)
                print("auth =", request_model.auth)

                response = await client.send(
                    request=request,
                    follow_redirects=request_options.follow_redirects,
                )
                print("response cookies =", response.cookies)
                self.post_message(HttpResponseReceived(response))

                script_context.response = response
                if on_response := request_model.scripts.on_response:
                    try:
                        self.get_and_run_script(
                            on_response,
                            "on_response",
                            response,
                            script_context,
                        )
                    except Exception:
                        self.response_script_output.set_response_status("error")
                        # TODO - load the error into the response area, or log it.
                    else:
                        self.response_script_output.set_response_status("success")
                else:
                    self.response_script_output.set_response_status("no-script")

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

    @work(group="send-request", exclusive=True)
    async def send_via_worker(self) -> None:
        if self.selected_request_type == "WEBSOCKET":
            print("Connecting to websocket")
            url_value = self.url_input.value
            await self.websocket_composer.send_message_or_connect(url_value)
        else:
            await self.send_request()

    @on(RequestTypeSelector.TypeChanged)
    def request_type_changed(self, event: RequestTypeSelector.TypeChanged) -> None:
        self.selected_request_type = event.value

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
        request = event.request
        if isinstance(request, HttpRequestModel):
            # TODO - move content switcher to http mode
            self.load_http_request_model(request)
        elif isinstance(request, WebsocketRequestModel):
            # TODO - move content switcher to websocket mode
            self.load_websocket_request_model(request)

        # TODO - ensure that the focus is set correctly for websocket requests
        # since some of the focus targets may not exist for websocket requests.
        if focus_on_request_open := self.settings.focus.on_request_open:
            targets = {
                "headers": self.headers_table,
                "body": self.request_editor.request_body_type_select,
                "query": self.request_editor.query_editor.query_key_input,
                "info": self.request_metadata.request_name_input,
                "url": self.url_input,
                "method": self.request_type_selector,
            }
            if target := targets.get(focus_on_request_open):
                self.set_focus(target)

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
        method_selector = self.request_type_selector
        method_selector.focus()
        method_selector.expanded = not method_selector.expanded

    def action_toggle_collection_browser(self) -> None:
        """Toggle the collection browser."""
        collection_browser = self.collection_browser
        collection_browser.display = not collection_browser.display

    def action_toggle_expanded(self) -> None:
        """Toggle the current expanded section.

        If a section is currently expanded, it will be collapsed.
        If no section is currently expanded, the currently focused section will be expanded.
        """
        if self.expanded_section in {"request", "response"}:
            self.expand_section(None)
        elif self.focused:
            # Expand the currently focused section.
            if self.focus_within_request():
                self.expand_section("request")
            elif self.focus_within_response():
                self.expand_section("response")

    def expand_section(self, section: Literal["request", "response"] | None) -> None:
        self.expanded_section = section

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
            request_model, HttpRequestModel
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

    def watch_current_layout(self, layout: Literal["horizontal", "vertical"]) -> None:
        """Update the current layout of the app to be horizontal or vertical."""
        classes = {"horizontal", "vertical"}
        other_class = classes.difference({layout}).pop()
        self.app_body.add_class(f"layout-{layout}")
        self.app_body.remove_class(f"layout-{other_class}")

    def watch_expanded_section(
        self, section: Literal["request", "response"] | None
    ) -> None:
        """Hide the non-expanded section."""
        request_editor = self.request_editor
        response_area = self.response_area

        # Hide the request editor if the response area is currently expanded,
        # and vice-versa.
        request_editor.set_class(section == "response", "hidden")
        response_area.set_class(section == "request", "hidden")

    def watch_selected_request_type(self, request_type: RequestType) -> None:
        body_content_switcher = self.query_one("#app-body-switcher")
        if request_type == "WEBSOCKET":
            body_content_switcher.current = "app-body-websocket-container"
            self.url_bar.mode = "realtime"
        else:
            body_content_switcher.current = "app-body-http-container"
            self.url_bar.mode = "http"

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
        request_model: HttpRequestModel,
        client: httpx.AsyncClient,
    ) -> httpx.Request:
        """Build an httpx request from the UI."""
        request = request_model.to_httpx(client)
        request.extensions = {"trace": self.log_request_trace_event}

        return request

    async def log_request_trace_event(self, event: Event, info: dict[str, Any]) -> None:
        """Log an event to the request trace."""
        await self.response_trace.log_event(event, info)
        self.url_bar.log_event(event, info)

    def build_request_model(self, request_options: Options) -> HttpRequestModel:
        """Grab data from the UI and pull it into a request model. This model
        may be passed around, stored on disk, etc."""
        open_node = self.collection_tree.currently_open
        open_request = open_node.data if open_node else None

        # We ensure elsewhere that the we can only "open" requests, not collection nodes.
        assert not isinstance(open_request, Collection)

        request_editor_args = self.request_editor.to_request_model_args()
        headers = self.headers_table.to_model()
        if request_body := request_editor_args.get("body"):
            header_names_lower = {header.name.lower(): header for header in headers}
            # Don't add the content type header if the user has explicitly set it.
            if (
                request_body.content_type is not None
                and "content-type" not in header_names_lower
            ):
                headers.append(
                    Header(
                        name="content-type",
                        value=request_body.content_type,
                    )
                )
        return HttpRequestModel(
            name=self.request_metadata.request_name,
            path=open_request.path if open_request else None,
            description=self.request_metadata.description,
            method=self.selected_request_type,
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
            scripts=self.request_scripts.to_model(),
            **request_editor_args,
        )

    @on(WebSocketConnected)
    def on_websocket_connected(self, event: WebSocketConnected) -> None:
        self.url_bar.websocket_connected_state()

    @on(WebSocketDisconnected)
    def on_websocket_disconnected(self, event: WebSocketDisconnected) -> None:
        self.url_bar.websocket_disconnected_state()

    def on_curl_message(self, event: CurlMessage):
        try:
            curl_import = CurlImport(event.curl_command)
            request_model = curl_import.to_request_model()
        except Exception:
            self.notify(
                title="Import error",
                message="Couldn't import curl command.",
                timeout=5,
                severity="error",
            )
        else:
            self.load_http_request_model(request_model)
            self.notify(
                title="Curl request imported",
                message=f"Successfully imported request to {curl_import.url}",
                timeout=3,
            )

    def load_websocket_request_model(
        self, request_model: WebsocketRequestModel
    ) -> None:
        """Load a request model into the UI."""
        self.selected_request_type = request_model.method
        self.request_type_selector.value = request_model.method
        self.url_input.value = str(request_model.url)

    def load_http_request_model(self, request_model: HttpRequestModel) -> None:
        """Load a request model into the UI."""
        self.selected_request_type = request_model.method
        self.request_type_selector.value = request_model.method
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
        self.request_scripts.load_scripts(request_model.scripts)

    def action_toggle_jump_mode(self) -> None:
        self._jumping = not self._jumping

    def watch__jumping(self, jumping: bool) -> None:
        focused_before = self.focused
        if focused_before is not None:
            self.set_focus(None, scroll_visible=False)

        def handle_jump_target(target: str | Widget | None) -> None:
            if isinstance(target, str):
                try:
                    target_widget = self.query_one(f"#{target}")
                except NoMatches:
                    log.warning(
                        f"Attempted to jump to target #{target}, but it couldn't be found on {self.screen!r}"
                    )
                else:
                    if target_widget.focusable:
                        self.set_focus(target_widget)
                    else:
                        target_widget.post_message(
                            Click(target_widget, 0, 0, 0, 0, 0, False, False, False)
                        )

            elif isinstance(target, Widget):
                self.set_focus(target)
            else:
                # If there's no target (i.e. the user pressed ESC to dismiss)
                # then re-focus the widget that was focused before we opened
                # the jumper.
                if focused_before is not None:
                    self.set_focus(focused_before, scroll_visible=False)

        self.app.clear_notifications()
        self.app.push_screen(JumpOverlay(self.jumper), callback=handle_jump_target)

    @property
    def url_bar(self) -> UrlBar:
        return self.query_one(UrlBar)

    @property
    def request_type_selector(self) -> RequestTypeSelector:
        return self.query_one(RequestTypeSelector)

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
    def request_scripts(self) -> RequestScripts:
        return self.query_one(RequestScripts)

    @property
    def collection_tree(self) -> CollectionTree:
        return self.query_one(CollectionTree)

    @property
    def response_trace(self) -> ResponseTrace:
        return self.query_one(ResponseTrace)

    @property
    def response_script_output(self) -> ScriptOutput:
        return self.query_one(ScriptOutput)

    @property
    def websocket_composer(self) -> WebsocketComposer:
        return self.query_one(WebsocketComposer)