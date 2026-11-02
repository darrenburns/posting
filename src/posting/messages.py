from dataclasses import dataclass

from httpx import Response
from textual.message import Message

from posting.collection import RequestModel


@dataclass
class HttpResponseReceived(Message):
    response: Response
    request: RequestModel | None = None
