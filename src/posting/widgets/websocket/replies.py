from dataclasses import dataclass
import datetime
from textual.app import ComposeResult
from textual.containers import Vertical
from textual.message import Message
from textual.widgets import Label


class Replies(Vertical):
    @dataclass
    class Incoming(Message):
        message: str
        timestamp: datetime.datetime

    def compose(self) -> ComposeResult:
        self.border_title = "Incoming"
        yield Label("Replies")
