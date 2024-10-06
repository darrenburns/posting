from dataclasses import dataclass
import httpx
from textual.events import Message


@dataclass
class HttpResponseReceived(Message):
    """Sent when an HTTP response is received."""

    response: httpx.Response


@dataclass
class RequestChanged(Message):
    """Sent from one of the request tabs when a change occurs within.

    We collect up any type of change from within a tab, so that at the
    MainScreen level we don't need to worry about the specific widgets
    that appear inside tabs.

    This also lets us manage "dirty" requests. When loading a new request,
    we can use `MessagePump.prevent(RequestChanged)` to load the new request
    without worrying about triggering our @on(RequestChanged) handler.

    Inside our @on(RequestChanged) handler we can set the dirty value as normal.
    """
