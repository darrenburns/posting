from dataclasses import dataclass
import httpx
from textual.events import Message


@dataclass
class HttpResponseReceived(Message):
    response: httpx.Response
