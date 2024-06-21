import time
from typing import Any, Literal
from textual.app import ComposeResult
from textual.containers import VerticalScroll
from textual.widgets import Label


Event = Literal[
    "connection.connect_tcp.started",
    "connection.connect_tcp.complete",
    "connection.connect_tcp.failed",
    "connection.connect_unix_socket.started",
    "connection.connect_unix_socket.complete",
    "connection.connect_unix_socket.failed",
    "connection.start_tls.started",
    "connection.start_tls.complete",
    "connection.start_tls.failed",
    "http11.send_request_headers.started",
    "http11.send_request_headers.complete",
    "http11.send_request_headers.failed",
    "http11.send_request_body.started",
    "http11.send_request_body.complete",
    "http11.send_request_body.failed",
    "http11.receive_response_body.started",
    "http11.receive_response_body.complete",
    "http11.receive_response_body.failed",
    "http11.response_closed.started",
    "http11.response_closed.complete",
    "http11.response_closed.failed",
]


class ResponseTrace(VerticalScroll):
    DEFAULT_CSS = """\
        ResponseTrace {
            padding: 0 2;
        }    
    """

    def __init__(
        self,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(name=name, id=id, classes=classes, disabled=disabled)
        self.events: dict[str, dict[str, float]] = {}

    def compose(self) -> ComposeResult:
        self.can_focus = False
        if len(self.events) == 0:
            yield Label("Send a request to view the trace.")
        else:
            for event_name, status_times in self.events.items():
                if "completed" in status_times:
                    duration_ns = status_times["completed"] - status_times["started"]
                    duration_ms = duration_ns / 1000000
                    yield Label(
                        f"[b]{event_name}[/b]: [green]{duration_ms:.2f}ms[/green]"
                    )
                elif "failed" in status_times:
                    yield Label(f"[b]{event_name}[/b]: [red]failed[/red]")
                else:
                    yield Label(f"[b]{event_name}[/b]: [yellow]waiting[/yellow]")

    async def log_event(self, event_name: Event, info: dict[str, Any]) -> None:
        event_name, status = event_name.rsplit(".", maxsplit=1)
        events = self.events
        match status:
            case "started":
                events[event_name] = {"started": time.perf_counter_ns()}
            case "complete":
                events[event_name]["completed"] = time.perf_counter_ns()
            case "failed":
                events[event_name]["failed"] = time.perf_counter_ns()
            case _:
                pass

        await self.recompose()

    def trace_complete(self) -> None:
        self.events = {}
