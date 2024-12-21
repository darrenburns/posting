from dataclasses import dataclass
import datetime
from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical, VerticalScroll
from textual.message import Message
from textual.reactive import reactive
from textual.widget import Widget
from textual.widgets import Label, RichLog, Static

from posting.widgets.text_area import ReadOnlyTextArea


class Replies(VerticalScroll):
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
        yield RichLog(id="replies-rich-log", highlight=True)

    def add_reply(self, message: Incoming) -> None:
        self.number_of_replies += 1
        self.mount(Reply(message, self.number_of_replies))

    def watch_number_of_replies(self, number: int) -> None:
        self.border_subtitle = f"{number} messages received"

    @property
    def replies_rich_log(self) -> RichLog:
        return self.query_one(RichLog)


class Reply(Vertical):
    def __init__(
        self,
        incoming: Replies.Incoming,
        number: int,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(name=name, id=id, classes=classes, disabled=disabled)
        self.incoming = incoming
        self.number = number

    def compose(self) -> ComposeResult:
        with Horizontal(classes="reply-header"):
            yield Label(f"#{self.number}", classes="reply-label")
            yield Label(
                self.incoming.timestamp.strftime("%H:%M:%S"),
                classes="reply-timestamp",
            )

        yield ReadOnlyTextArea(self.incoming.message, classes="reply-text-area")
