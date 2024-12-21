from dataclasses import dataclass
import datetime
from textual.app import ComposeResult
from textual.containers import Vertical
from textual.message import Message
from textual.widgets import Label, RichLog


class Replies(Vertical):
    @dataclass
    class Incoming(Message):
        message: str
        timestamp: datetime.datetime

    def compose(self) -> ComposeResult:
        self.border_title = "Incoming"
        yield Label("Replies")
        yield RichLog(id="replies-rich-log")

    def add_reply(self, message: str) -> None:
        self.replies_rich_log.write(message)

    @property
    def replies_rich_log(self) -> RichLog:
        return self.query_one(RichLog)
