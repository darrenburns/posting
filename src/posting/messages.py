from dataclasses import dataclass
from httpx import Response
from textual.message import Message


@dataclass
class HttpResponseReceived(Message):
    response: Response
