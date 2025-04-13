from dataclasses import dataclass
from typing import Any
import httpx
from rich.text import Text
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.css.query import NoMatches
from textual.events import Paste
from textual.message import Message
from textual.reactive import Reactive, reactive
from textual.widgets import Input, Button, Label
from textual.theme import Theme
from textual.widgets.input import Selection
from textual_autocomplete import DropdownItem, TargetState
from posting.config import SETTINGS
from posting.help_data import HelpData

from posting.highlighters import VariablesAndUrlHighlighter
from posting.themes import UrlStyles, VariableStyles
from posting.variables import (
    extract_variable_name,
    get_variable_at_cursor,
    get_variables,
)
from posting.widgets.input import PostingInput
from posting.widgets.request.method_selection import MethodSelector
from posting.widgets.response.response_trace import Event
from posting.widgets.variable_autocomplete import VariableAutoComplete
from posting.urls import ensure_protocol


class CurlMessage(Message):
    def __init__(self, curl_command: str) -> None:
        super().__init__()
        self.curl_command = curl_command


class UrlInput(PostingInput):
    """
    The URL input.
    """

    help = HelpData(
        "Address Bar",
        """\
Enter the URL to send a request to. Refer to variables from the environment (loaded via `--env`) using `$variable` or `${variable}` syntax.
Resolved variables will be highlighted green. Move the cursor over a variable to preview the value.
Base URL suggestions are loaded based on the URLs found in the currently open collection.
Press `ctrl+l` to quickly focus this bar from elsewhere.

You can also import a `curl` command by pasting it into the URL bar.
This will fill out the request details in the UI based on the curl command you pasted, overwriting any existing values.
It's recommended you create a new request before pasting a curl command, to avoid overwriting.
""",
    )

    BINDING_GROUP_TITLE = "URL Input"

    BINDINGS = [
        Binding("down", "app.focus_next", "Focus next", show=False),
    ]

    @dataclass
    class CursorMoved(Message):
        cursor_position: int
        value: str
        input: "UrlInput"

        @property
        def control(self) -> "UrlInput":
            return self.input

    def on_mount(self):
        self.highlighter = VariablesAndUrlHighlighter(self)
        self.app.theme_changed_signal.subscribe(self, self.on_theme_change)

    @on(Input.Changed)
    def on_change(self, event: Input.Changed) -> None:
        self.remove_class("error")

    def watch_selection(self, selection: Selection) -> None:
        self.post_message(self.CursorMoved(selection.end, self.value, self))

    def on_theme_change(self, theme: Theme) -> None:
        super().on_theme_change(theme)
        theme_variables = self.app.theme_variables
        self.highlighter.variable_styles = VariableStyles(
            resolved=theme_variables.get("variable-resolved")
            or theme_variables.get("text-success"),
            unresolved=theme_variables.get("variable-unresolved")
            or theme_variables.get("text-error"),
        )

        self.highlighter.url_styles = UrlStyles(
            base=theme_variables.get("url-base")
            or theme_variables.get("text-secondary"),
            protocol=theme_variables.get("url-protocol")
            or theme_variables.get("text-accent"),
            separator=theme_variables.get("url-separator")
            or theme_variables.get("foreground-muted"),
        )

    def on_paste(self, event: Paste):
        if not event.text.startswith("curl "):
            return
        event.prevent_default()
        self.post_message(CurlMessage(event.text))


class SendRequestButton(Button, can_focus=False):
    """
    The button for sending the request.
    """


class UrlBar(Vertical):
    """
    The URL bar.
    """

    COMPONENT_CLASSES = {
        "started-marker",
        "complete-marker",
        "failed-marker",
        "not-started-marker",
    }

    response_status_code: Reactive[int | None] = reactive(
        None, init=False, always_update=True
    )
    response_reason_phrase: Reactive[str | None] = reactive(
        None, init=False, always_update=True
    )

    def __init__(
        self,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(name=name, id=id, classes=classes, disabled=disabled)
        self.cached_base_urls: list[str] = []
        self._trace_events: set[Event] = set()

    def on_env_changed(self, _: None) -> None:
        self._display_variable_at_cursor()
        self.url_input.refresh()

    def watch_response_status_code(self, status_code: int | None) -> None:
        if status_code is None:
            return

        status_code_label = self.status_code_label
        status_code_label.remove_class("-success", "-warning", "-error")
        if status_code < 300:
            status_code_label.add_class("-success")
        elif status_code < 400:
            status_code_label.add_class("-warning")
        else:
            status_code_label.add_class("-error")

        status_code_label.update(str(status_code))
        status_code_label.display = True

    def watch_response_reason_phrase(self, reason_phrase: str | None) -> None:
        if reason_phrase is None:
            return

        self.status_code_label.tooltip = reason_phrase

    def compose(self) -> ComposeResult:
        with Horizontal(id="main-row"):
            yield MethodSelector(id="method-selector")
            yield UrlInput(
                placeholder="Enter a URL or paste a curl command…",
                id="url-input",
            )
            yield Label(id="response-status-code")
            yield Label(id="trace-markers")
            yield SendRequestButton("Send")

        variable_value_bar = Label(id="variable-value-bar")
        if not SETTINGS.get().url_bar.show_value_preview:
            variable_value_bar.styles.display = "none"
        yield variable_value_bar

    def on_mount(self) -> None:
        self.auto_complete = VariableAutoComplete(
            target=self.query_one("#url-input", UrlInput),
            candidates=self._get_autocomplete_candidates,
            variable_candidates=self._get_variable_candidates,
        )
        self.screen.mount(self.auto_complete)

        self.on_theme_change(self.app.current_theme)
        self.app.theme_changed_signal.subscribe(self, self.on_theme_change)
        self.app.env_changed_signal.subscribe(self, self.on_env_changed)

    @on(Input.Changed)
    def on_change(self, event: Input.Changed) -> None:
        try:
            self.variable_value_bar.update("")
        except NoMatches:
            return

    @on(Input.Blurred)
    def on_blur(self, event: Input.Blurred) -> None:
        try:
            self.variable_value_bar.update("")
        except NoMatches:
            return

    @on(UrlInput.CursorMoved)
    def on_cursor_moved(self, event: UrlInput.CursorMoved) -> None:
        self._display_variable_at_cursor()

    def _display_variable_at_cursor(self) -> None:
        url_input = self.url_input

        cursor_position = url_input.cursor_position
        value = url_input.value
        variable_at_cursor = get_variable_at_cursor(cursor_position, value)
        variables = get_variables()
        try:
            variable_bar = self.variable_value_bar
        except NoMatches:
            # Can be hidden with config, which will set display = None.
            # In this case, the query will fail.
            return

        if not variable_at_cursor:
            variable_bar.update("")
            return

        variable_name = extract_variable_name(variable_at_cursor)
        variable_value = variables.get(variable_name)
        if variable_value:
            if SETTINGS.get().url_bar.hide_secrets_in_value_preview:
                if any(
                    word in variable_name.lower()
                    for word in ["secret", "key", "password", "token"]
                ):
                    variable_value = "(hidden)"
            content = f"{variable_name} = {variable_value}"
            variable_bar.update(content)
        else:
            variable_bar.update("")

    def _get_autocomplete_candidates(
        self, target_state: TargetState
    ) -> list[DropdownItem]:
        return [DropdownItem(main=base_url) for base_url in self.cached_base_urls]

    def _get_variable_candidates(self, target_state: TargetState) -> list[DropdownItem]:
        return [DropdownItem(main=f"${variable}") for variable in get_variables()]

    def on_theme_change(self, theme: Theme) -> None:
        markers = self._build_markers()
        self.trace_markers.update(markers)
        self.url_input.notify_style_update()
        self.url_input.refresh()

    def log_event(self, event: Event, info: dict[str, Any]) -> None:
        """Log an event to the request trace."""
        self._trace_events.add(event)
        markers = self._build_markers()
        self.trace_markers.update(markers)

        self.trace_markers.set_class(len(self._trace_events) > 0, "has-events")

    def _build_markers(self) -> Text:
        def get_marker(event_base: str) -> Text:
            if f"{event_base}.complete" in self._trace_events:
                style = self.get_component_rich_style("complete-marker")
                return Text("■", style=style)
            elif f"{event_base}.failed" in self._trace_events:
                style = self.get_component_rich_style("failed-marker")
                return Text("■", style=style)
            elif f"{event_base}.started" in self._trace_events:
                style = self.get_component_rich_style("started-marker")
                return Text("■", style=style)
            else:
                style = self.get_component_rich_style("not-started-marker")
                return Text("■", style=style)

        event_bases = [
            "connection.connect_tcp",
            "connection.start_tls",
            "http11.send_request_headers",
            "http11.send_request_body",
            "http11.receive_response_headers",
            "http11.receive_response_body",
            "http11.response_closed",
        ]

        markers = {event: get_marker(event) for event in event_bases}

        return Text.assemble(*markers.values())

    def clear_events(self) -> None:
        """Clear the events from the request trace."""
        self.trace_markers.update("")
        self._trace_events.clear()

    @property
    def trace_markers(self) -> Label:
        """Get the trace markers."""
        return self.query_one("#trace-markers", Label)

    @property
    def variable_value_bar(self) -> Label:
        """Get the variable value bar."""
        return self.query_one("#variable-value-bar", Label)

    @property
    def url_input(self) -> UrlInput:
        """Get the URL input."""
        return self.query_one("#url-input", UrlInput)

    @property
    def status_code_label(self) -> Label:
        """Get the status code label."""
        return self.query_one("#response-status-code", Label)
