from dataclasses import dataclass
from typing import Any
from rich.text import Text
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.css.query import NoMatches
from textual.design import ColorSystem
from textual.events import Blur
from textual.message import Message
from textual.widgets import Input, Button, Label
from textual_autocomplete import DropdownItem
from textual_autocomplete._autocomplete2 import TargetState
from posting.config import SETTINGS
from posting.help_screen import HelpData

from posting.highlighters import VariablesAndUrlHighlighter
from posting.variables import (
    extract_variable_name,
    get_variable_at_cursor,
    get_variables,
)
from posting.widgets.input import PostingInput
from posting.widgets.request.method_selection import MethodSelector
from posting.widgets.response.response_trace import Event
from posting.widgets.variable_autocomplete import VariableAutoComplete


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
Press `ctrl+l` to quickly focus this bar from elsewhere.""",
    )

    DEFAULT_CSS = """\
    UrlInput {
        border: none;
        width: 1fr;
        &:focus {
            border: none;
            padding: 0 1;
            & .input--cursor {
              color: $text;
              background: $accent-lighten-2;
            }
        }
        &.error {
            border-left: thick $error;
        }
    }
    """

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

    @dataclass
    class Blurred(Message):
        input: "UrlInput"

        @property
        def control(self) -> "UrlInput":
            return self.input

    def on_mount(self):
        self.highlighter = VariablesAndUrlHighlighter(self)

    @on(Input.Changed)
    def on_change(self, event: Input.Changed) -> None:
        self.remove_class("error")

    def on_blur(self, _: Blur) -> None:
        self.post_message(self.Blurred(self))

    def watch_cursor_position(self, cursor_position: int) -> None:
        self.post_message(self.CursorMoved(cursor_position, self.value, self))


class SendRequestButton(Button, can_focus=False):
    """
    The button for sending the request.
    """

    DEFAULT_CSS = """\
    SendRequestButton {
        padding: 0 1;
        height: 1;
        min-width: 10;
        background: $primary;
        color: $text;
        border: none;
        text-style: none;
        &:hover {
            text-style: b;
            padding: 0 1;
            border: none;
            background: $primary-darken-1;
        }
    }
    """


class UrlBar(Vertical):
    """
    The URL bar.
    """

    DEFAULT_CSS = """\
    UrlBar {
        height: 3;
        padding: 1 3 0 3;

        & #trace-markers {
            padding: 0 1;
            display: none;
            background: $surface;

            &.has-events {
                display: block;
                width: auto;
            }
        }
        & #variable-value-bar {
            width: 1fr;
            color: $text-muted;
            text-align: center;
            height: 1;
        }
        & .complete-marker {
            color: $success;
            background: $surface;
        }
        & .failed-marker {
            color: $error;
            background: $surface;
        }
        & .started-marker {
            color: $warning;
            background: $surface;
        }
        & .not-started-marker {
            color: $text-muted 30%;
            background: $surface;
        }
    }
    """

    COMPONENT_CLASSES = {
        "started-marker",
        "complete-marker",
        "failed-marker",
        "not-started-marker",
    }

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

    def compose(self) -> ComposeResult:
        with Horizontal():
            yield MethodSelector(id="method-selector")
            yield UrlInput(
                placeholder="Enter a URL...",
                id="url-input",
            )
            yield Label(id="trace-markers")
            yield SendRequestButton("Send")
        variable_value_bar = Label(id="variable-value-bar")
        if SETTINGS.get().url_bar.show_value_preview:
            yield variable_value_bar

    def on_mount(self) -> None:
        self.auto_complete = VariableAutoComplete(
            target=self.query_one("#url-input", UrlInput),
            candidates=self._get_autocomplete_candidates,
            variable_candidates=self._get_variable_candidates,
        )
        self.screen.mount(self.auto_complete)
        self.app.theme_change_signal.subscribe(self, self.on_theme_change)

    @on(Input.Changed)
    def on_change(self, event: Input.Changed) -> None:
        self.variable_value_bar.update("")

    @on(UrlInput.Blurred)
    def on_blur(self, event: UrlInput.Blurred) -> None:
        self.variable_value_bar.update("")

    @on(UrlInput.CursorMoved)
    def on_cursor_moved(self, event: UrlInput.CursorMoved) -> None:
        variables = get_variables()
        variable_at_cursor = get_variable_at_cursor(event.cursor_position, event.value)
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

    def on_theme_change(self, theme: ColorSystem) -> None:
        markers = self._build_markers()
        self.trace_markers.update(markers)

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
