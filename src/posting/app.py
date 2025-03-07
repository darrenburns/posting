import inspect
from contextlib import redirect_stdout, redirect_stderr
import os
from pathlib import Path
import sys
from typing import Any, Literal, cast

import httpx
from rich.console import RenderableType
from textual.content import Content

from posting.importing.curl import CurlImport
from textual import messages, on, log, work
from textual.command import CommandPalette, SimpleCommand
from textual.css.query import NoMatches
from textual.events import Click
from textual.reactive import Reactive, reactive
from textual.app import App, ComposeResult, InvalidThemeError
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.screen import Screen
from textual.markup import escape
from textual.signal import Signal
from textual.theme import Theme, BUILTIN_THEMES as TEXTUAL_THEMES
from textual.widget import Widget
from textual.widgets import (
    Button,
    Footer,
    Input,
    Label,
    TextArea,
)
from textual.widgets._tabbed_content import ContentTab
from watchfiles import Change, awatch
from posting.collection import (
    Collection,
    Cookie,
    Header,
    HttpRequestMethod,
    Options,
    RequestModel,
)

from posting.commands import PostingProvider
from posting.config import SETTINGS, Settings
from posting.help_screen import HelpScreen
from posting.jump_overlay import JumpOverlay
from posting.jumper import Jumper
from posting.scripts import execute_script, uncache_module, Posting as PostingContext
from posting.themes import (
    BUILTIN_THEMES,
    load_user_theme,
    load_user_themes,
)
from posting.types import CertTypes, PostingLayout
from posting.user_host import get_user_host_string
from posting.variables import (
    SubstitutionError,
    get_variables,
    load_variables,
    update_variables,
)
from posting.version import VERSION
from posting.widgets.collection.browser import (
    CollectionBrowser,
    CollectionTree,
)
from posting.widgets.datatable import PostingDataTable
from posting.widgets.request.header_editor import HeadersTable
from posting.messages import HttpResponseReceived
from posting.widgets.request.method_selection import MethodSelector

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
from posting.xresources import load_xresources_themes


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


class MainScreen(Screen[None]):
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
            "ctrl+h",
            "toggle_collection_browser",
            "Toggle collection browser",
            show=False,
            tooltip="Toggle the collection browser.",
            id="toggle-collection",
        ),
        Binding(
            "ctrl+P,ctrl+shift+p",
            "open_request_search_palette",
            "Search requests",
            show=True,
            tooltip="Search for a request by name.",
            id="search-requests",
        ),
    ]

    selected_method: Reactive[HttpRequestMethod] = reactive("GET", init=False)
    """The currently selected method of the request."""
    current_layout: Reactive[PostingLayout] = reactive("vertical", init=False)
    """The current layout of the app."""
    expanded_section: Reactive[Literal["request", "response"] | None] = reactive(
        None, init=False
    )
    """The currently expanded section of the main screen."""

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
        self.current_layout = self._initial_layout

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
            collection_browser = CollectionBrowser(collection=self.collection)
            collection_browser.display = (
                self.settings.collection_browser.show_on_startup
            )
            yield collection_browser
            yield RequestEditor()
            yield ResponseArea()
        yield Footer(show_command_palette=False)

    def get_and_run_script(
        self,
        path_to_script: str,
        default_function_name: str,
        write_logs_to_ui: bool = True,
        *args: Any,
    ) -> None:
        """
        Get and run a function from a script.

        Args:
            path_to_script: Path to the script, relative to the collection path.
            default_function_name: Default function name to use if not specified in the path.
            write_logs_to_ui: Whether to write logs to the UI.
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

                if write_logs_to_ui:
                    stdout_log = RichLogIO(rich_log, "stdout")
                    stderr_log = RichLogIO(rich_log, "stderr")
                else:
                    stdout_log = sys.stdout
                    stderr_log = sys.stderr
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
                        True,
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
                            True,
                            # The args below are passed to the script function.
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
                response = await client.send(
                    request=request,
                    follow_redirects=request_options.follow_redirects,
                )

                self.post_message(HttpResponseReceived(response))

                script_context.response = response
                if on_response := request_model.scripts.on_response:
                    try:
                        self.get_and_run_script(
                            on_response,
                            "on_response",
                            True,
                            # The args below are passed to the script function.
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
                message=f"Couldn't connect within {request_options.timeout} seconds.",
            )
        except httpx.ReadTimeout as read_timeout:
            log.error("Read timeout", read_timeout)
            self.notify(
                severity="error",
                title="Read timeout",
                message=f"Couldn't read data within {request_options.timeout} seconds.",
            )
        except httpx.WriteTimeout as write_timeout:
            log.error("Write timeout", write_timeout)
            self.notify(
                severity="error",
                title="Write timeout",
                message=f"Couldn't send data within {request_options.timeout} seconds.",
            )
        except Exception as e:
            log.error("Error sending request", e)
            log.error("Type of error", type(e))
            self.url_input.add_class("error")
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
        if focus_on_request_open := self.settings.focus.on_request_open:
            targets = {
                "headers": self.headers_table,
                "body": self.request_editor.request_body_type_select,
                "query": self.request_editor.query_editor.query_key_input,
                "info": self.request_metadata.request_name_input,
                "url": self.url_input,
                "method": self.method_selector,
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
        method_selector = self.method_selector
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
        assert isinstance(request_model, RequestModel), (
            "currently open node should contain a request model"
        )

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
        request_model: RequestModel,
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

    def build_request_model(self, request_options: Options) -> RequestModel:
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
            scripts=self.request_scripts.to_model(),
            **request_editor_args,
        )

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
            self.load_request_model(request_model, overwrite_metadata=False)
            self.notify(
                title="Curl request imported",
                message=f"Successfully imported request to {curl_import.url}",
                timeout=3,
            )

    def action_open_request_search_palette(self) -> None:
        """Open the request search palette."""
        collection_tree_nodes = list(self.collection_tree.walk_nodes())

        def load_and_select_request(request: RequestModel) -> None:
            self.load_request_model(request)
            for node in collection_tree_nodes:
                if node.data == request:
                    self.collection_tree.select_node(node)
                    break

        collection_path = self.collection.path
        self.app.search_commands(
            [
                SimpleCommand(
                    name=node.data.name if node.data.path else node.data.name,
                    callback=lambda request=node.data: load_and_select_request(request),
                    help_text=(
                        str(node.data.path.relative_to(collection_path))
                        if node.data.path
                        else ""
                    ),
                )
                for node in collection_tree_nodes
                if isinstance(node.data, RequestModel)
            ],
            placeholder="Search for a request…",
        )

    def load_request_model(
        self, request_model: RequestModel, overwrite_metadata: bool = True
    ) -> None:
        """Load a request model into the UI.

        Args:
            request_model: The request model to load.
            overwrite_metadata: Whether to overwrite the metadata of the currently
                open request. If False, only the request data will be loaded, and
                the metadata (name, description, etc) will be left as is.
        """
        self.selected_method = request_model.method
        self.method_selector.value = request_model.method
        self.url_input.value = str(request_model.url)
        self.params_table.replace_all_rows(
            ((param.name, param.value) for param in request_model.params),
            (param.enabled for param in request_model.params),
        )
        self.headers_table.replace_all_rows(
            ((header.name, header.value) for header in request_model.headers),
            (header.enabled for header in request_model.headers),
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
                    (
                        (param.name, param.value)
                        for param in request_model.body.form_data
                    ),
                    (param.enabled for param in request_model.body.form_data),
                )
                self.request_editor.request_body_type_select.value = "form-body-editor"
                self.request_body_text_area.text = ""
        else:
            self.request_body_text_area.text = ""
            self.request_editor.form_editor.replace_all_rows([])
            self.request_editor.request_body_type_select.value = "no-body-label"

        if overwrite_metadata:
            # Sometimes we don't wish to write request metadata, for example, if we're
            # importing a request from curl, the user may have already written their
            # own metadata - we should not replace that.
            self.request_metadata.request = request_model

        self.request_options.load_options(request_model.options)
        self.request_auth.load_auth(request_model.auth)
        self.request_scripts.load_scripts(request_model.scripts)

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


class Posting(App[None], inherit_bindings=False):
    ALLOW_SELECT = False
    AUTO_FOCUS = None
    COMMANDS = {PostingProvider}
    CSS_PATH = Path(__file__).parent / "posting.scss"
    BINDING_GROUP_TITLE = "Global Keybinds"
    BINDINGS = [
        Binding(
            "ctrl+p",
            "command_palette",
            description="Commands",
            tooltip="Open the command palette to search and run commands.",
            id="commands",
        ),
        Binding(
            "ctrl+o",
            "toggle_jump_mode",
            description="Jump",
            tooltip="Activate jump mode to quickly move focus between widgets.",
            id="jump",
        ),
        Binding(
            "ctrl+c",
            "app.quit",
            description="Quit",
            tooltip="Quit the application.",
            priority=True,
            id="quit",
        ),
        Binding(
            "f1,ctrl+question_mark,ctrl+shift+slash",
            "help",
            "Help",
            tooltip="Open the help dialog for the currently focused widget.",
            id="help",
        ),
        Binding("f8", "save_screenshot", "Save screenshot.", show=False),
    ]

    def __init__(
        self,
        settings: Settings,
        environment_files: tuple[Path, ...],
        collection: Collection,
        collection_specified: bool = False,
    ) -> None:
        SETTINGS.set(settings)

        self.settings = settings
        """Settings object which is built via pydantic-settings,
        essentially a direct translation of the config.yaml file."""

        self.environment_files = environment_files
        """A list of paths to dotenv files, in the order they're loaded."""

        self.collection = collection
        """The loaded collection."""

        self.collection_specified = collection_specified
        """Boolean indicating whether the user launched Posting explicitly
        supplying a collection directory, or if they let Posting auto-discover
        it in some way (likely just using the default collection)."""

        self.env_changed_signal = Signal[None](self, "env-changed")
        """Signal that is published when the environment has changed.
        This means one or more of the loaded environment files (in
        `self.environment_files`) have been modified."""

        self.session_env: dict[str, object] = {}
        """Users can set the value of variables for the duration of the
        session (until the app is quit). This can be done via the scripting
        interface: pre-request or post-response scripts."""

        super().__init__()

        # The animation is set AFTER the app is initialized intentionally,
        # as it needs to override the default approach taken by Textual in
        # App.__init__().
        self.animation_level = settings.animation
        """The level of animation to use in the app. This is used by Textual."""

    _jumping: Reactive[bool] = reactive(False, init=False, bindings=True)
    """True if 'jump mode' is currently active, otherwise False."""

    @work(exclusive=True, group="environment-watcher")
    async def watch_environment_files(self) -> None:
        """Watching files that were passed in as the environment."""
        async for changes in awatch(*self.environment_files):
            # Reload the variables from the environment files.
            load_variables(
                self.environment_files,
                self.settings.use_host_environment,
                avoid_cache=True,
            )
            # Overlay the session variables on top of the environment variables.
            update_variables(self.session_env)

            # Notify the app that the environment has changed,
            # which will trigger a reload of the variables in the relevant widgets.
            # Widgets subscribed to this signal can reload as needed.
            # For example, AutoComplete dropdowns will want to reload their
            # candidate variables when the environment changes.
            self.env_changed_signal.publish(None)
            self.notify(
                title="Environment changed",
                message=f"Reloaded {len(changes)} dotenv files",
                timeout=3,
            )

    @work(exclusive=True, group="collection-watcher")
    async def watch_collection_files(self) -> None:
        """Watching specific files within the collection directory."""
        async for changes in awatch(self.collection.path):
            for change_type, file_path in changes:
                if file_path.endswith(".py"):
                    if change_type in (
                        Change.deleted,
                        Change.modified,
                    ):
                        # If a Python file was updated, then we want to clear
                        # the script module cache for the app so that modules
                        # are reloaded on the next request being sent.
                        # Without this, we'd hit the module cache and simply
                        # re-execute the previously cached module.
                        uncache_module(file_path)
                        file_path_object = Path(file_path)
                        file_name = file_path_object.name
                        self.notify(
                            f"Reloaded {file_name!r}",
                            title="Script reloaded",
                            timeout=2,
                        )
                    if change_type in (Change.added, Change.deleted):
                        # TODO - update the autocompletion
                        # of the available scripts.
                        pass

    @work(exclusive=True, group="theme-watcher")
    async def watch_themes(self) -> None:
        """Watching the theme directory for changes."""
        async for changes in awatch(self.settings.theme_directory):
            for _change_type, file_path in changes:
                if file_path.endswith((".yml", ".yaml")):
                    try:
                        theme = load_user_theme(Path(file_path))
                    except Exception as e:
                        print(f"Couldn't load theme from {str(file_path)}: {e}.")
                        continue
                    if theme and theme.name == self.theme:
                        self.register_theme(theme)
                        self.set_reactive(App.theme, theme.name)
                        try:
                            self._watch_theme(theme.name)
                        except Exception as e:
                            # I don't think we want to notify here, as editors often
                            # use heuristics to determine whether to save a file. This could
                            # prove jarring if we pop up a notification without the user
                            # explicitly saving the file in their editor.
                            print(f"Error refreshing CSS: {e}")

    def on_mount(self) -> None:
        settings = SETTINGS.get()

        available_themes: dict[str, Theme] = {"galaxy": BUILTIN_THEMES["galaxy"]}

        if settings.load_builtin_themes:
            available_themes |= BUILTIN_THEMES
        else:
            for theme in TEXTUAL_THEMES.values():
                self.unregister_theme(theme.name)

        if settings.use_xresources:
            available_themes |= load_xresources_themes()

        if settings.load_user_themes:
            loaded_themes, failed_themes = load_user_themes()
            available_themes |= loaded_themes

            # Display a single message for all failed themes.
            if failed_themes:
                self.notify(
                    "\n".join(f"• {path.name}" for path, _ in failed_themes),
                    title=f"Failed to read {len(failed_themes)} theme{'s' if len(failed_themes) > 1 else ''}",
                    severity="error",
                    timeout=8,
                )

        for theme in available_themes.values():
            self.register_theme(theme)

        unwanted_themes = [
            "textual-ansi",
        ]
        for theme_name in unwanted_themes:
            self.unregister_theme(theme_name)

        try:
            self.theme = settings.theme
        except InvalidThemeError:
            # This can happen if the user has a custom theme that is invalid,
            # e.g. a color is invalid or the YAML cannot be parsed.
            self.theme = "galaxy"
            self.notify(
                "Check theme file for syntax errors, invalid colors, etc.\n"
                "Falling back to [b i]galaxy[/] theme.",
                title=f"Couldn't apply theme {settings.theme!r}",
                severity="error",
                timeout=8,
            )

        self.set_keymap(self.settings.keymap)
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
            screen=self.screen,
        )
        if self.settings.watch_env_files:
            self.watch_environment_files()

        if self.settings.watch_collection_files:
            self.watch_collection_files()

        if self.settings.watch_themes:
            self.watch_themes()

    def get_default_screen(self) -> MainScreen:
        self.main_screen = MainScreen(
            collection=self.collection,
            layout=self.settings.layout,
            environment_files=self.environment_files,
        )
        return self.main_screen

    def command_layout(self, layout: Literal["vertical", "horizontal"]) -> None:
        self.main_screen.current_layout = layout

    def command_export_to_curl(self, run_setup_scripts: bool = True) -> None:
        main_screen = self.main_screen
        request_model = main_screen.build_request_model(
            main_screen.request_options.to_model()
        )

        # There are two options in the command palette for exporting to curl.
        # Users can choose to run setup scripts attached to the request in order to
        # set variables etc. or they can choose to skip setup scripts entirely.
        if run_setup_scripts and (setup_script := request_model.scripts.setup):
            try:
                main_screen.get_and_run_script(
                    setup_script,
                    "setup",
                    False,
                    PostingContext(self),
                )
            except Exception:
                self.notify(
                    "Error running setup script",
                    title="Error in script",
                    severity="error",
                )

        variables = get_variables()
        try:
            request_model.apply_template(variables)
        except SubstitutionError as e:
            log.error(e)
            self.notify(
                str(e),
                title="Undefined variable",
                severity="warning",
            )
            pass

        curl_command = request_model.to_curl(
            extra_args=self.settings.curl_export_extra_args
        )

        if os.getenv("TERM_PROGRAM") == "Apple_Terminal":
            # Apple terminal doesn't support OSC 52, so we need to use
            # pyperclip to copy the command to the clipboard.
            try:
                import pyperclip

                pyperclip.copy(curl_command)
            except pyperclip.PyperclipException as exc:
                self.notify(
                    str(exc),
                    title="Clipboard error",
                    severity="error",
                    timeout=10,
                )
            else:
                self.notify(escape(curl_command), title="Copied to clipboard")
        else:
            self.notify(
                escape(curl_command),
                title="Copied to clipboard",
            )
            self.app.copy_to_clipboard(curl_command)

    def action_save_screenshot(
        self,
    ) -> str:
        return self.save_screenshot()

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

        prompt = event.highlighted_event.option.prompt
        themes = self.available_themes.keys()
        if isinstance(prompt, Content):
            candidate = prompt.plain
            if candidate in themes:
                self.theme = candidate
            else:
                self.theme = self._original_theme
            self.call_next(self.screen._update_styles)

    @on(CommandPalette.Closed)
    def palette_closed(self, event: CommandPalette.Closed) -> None:
        # If we closed with a result, that will be handled by the command
        # being triggered. However, if we closed the palette with no result
        # then make sure we revert the theme back.
        if not self.settings.command_palette.theme_preview:
            return
        if not event.option_selected:
            self.theme = self._original_theme

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

        self.clear_notifications()
        self.push_screen(JumpOverlay(self.jumper), callback=handle_jump_target)

    async def action_help(self) -> None:
        focused = self.focused

        def reset_focus(_) -> None:
            if focused:
                self.screen.set_focus(focused)

        self.set_focus(None)
        await self.push_screen(HelpScreen(widget=focused), callback=reset_focus)

    def exit(
        self,
        result: object | None = None,
        return_code: int = 0,
        message: RenderableType | None = None,
    ) -> None:
        """Exit the app, and return the supplied result.

        Args:
            result: Return value.
            return_code: The return code. Use non-zero values for error codes.
            message: Optional message to display on exit.
        """
        self._exit = True
        self._return_value = result
        self._return_code = return_code
        self.post_message(messages.ExitApp())
        if message:
            self._exit_renderables.append(message)
            self._exit_renderables = list(set(self._exit_renderables))
