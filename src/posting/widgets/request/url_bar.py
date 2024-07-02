from typing import Any
from rich.text import Text
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal
from textual.design import ColorSystem
from textual.widgets import Input, Button, Label
from textual_autocomplete import AutoComplete, DropdownItem
from textual_autocomplete._autocomplete2 import TargetState

from posting.highlight_url import URLHighlighter
from posting.widgets.request.method_selection import MethodSelection
from posting.widgets.response.response_trace import Event


class UrlInput(Input):
    """
    The URL input.
    """

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

    def on_mount(self):
        self.highlighter = URLHighlighter()

    @on(Input.Changed)
    def on_change(self, event: Input.Changed) -> None:
        self.remove_class("error")


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


class UrlBar(Horizontal):
    """
    The URL bar.
    """

    DEFAULT_CSS = """\
    UrlBar {
        height: 3;
        padding: 1 3 0 3;

        & > #trace-markers {
            padding: 0 1;
            display: none;
            background: $surface;

            &.has-events {
                display: block;
                width: auto;
            }
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
        yield MethodSelection("GET")
        yield UrlInput(
            placeholder="Enter a URL...",
            id="url-input",
        )
        yield Label(id="trace-markers")
        yield SendRequestButton("Send")

    def on_mount(self) -> None:
        self.auto_complete = AutoComplete(
            target=self.query_one("#url-input", UrlInput),
            items=self._get_autocomplete_items,
        )
        self.screen.mount(self.auto_complete)
        self.app.theme_change_signal.subscribe(self, self.on_theme_change)

    def on_theme_change(self, theme: ColorSystem) -> None:
        print("theme change")
        markers = self._build_markers()
        self.trace_markers.update(markers)

    def _get_autocomplete_items(self, target_state: TargetState) -> list[DropdownItem]:
        return [DropdownItem(main=base_url) for base_url in self.cached_base_urls]

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
        print("cleared events")

    @property
    def trace_markers(self) -> Label:
        """Get the trace markers."""
        return self.query_one("#trace-markers", Label)
