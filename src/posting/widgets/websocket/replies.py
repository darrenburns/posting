from dataclasses import dataclass
import datetime
from textual.app import ComposeResult
from textual.containers import Vertical
from textual.message import Message
from textual.reactive import reactive
from textual.widgets import Label, RichLog


class Replies(Vertical):
    """
    A widget for displaying replies from the server.
    """

    number_of_replies: int = reactive(0)

    @dataclass
    class Incoming(Message):
        message: str
        timestamp: datetime.datetime

    def compose(self) -> ComposeResult:
        self.border_title = "Incoming"
        yield RichLog(id="replies-rich-log")

    def add_reply(self, message: str) -> None:
        self.replies_rich_log.write(message)
        self.number_of_replies += 1

    def watch_number_of_replies(self, number: int) -> None:
        self.border_subtitle = f"{number} messages received"

    @property
    def replies_rich_log(self) -> RichLog:
        return self.query_one(RichLog)
