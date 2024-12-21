from dataclasses import dataclass
import datetime
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical, VerticalScroll
from textual.message import Message
from textual.reactive import reactive
from textual.widgets import Label, Sparkline
from textual.widgets.text_area import Selection

from posting.widgets.text_area import ReadOnlyTextArea


class RepliesFooter(Horizontal):
    auto_scroll: bool = reactive(True)

    def compose(self) -> ComposeResult:
        yield Label(
            "Auto Scroll [b]ON[/]", id="replies-auto-scroll-label", classes="on"
        )
        yield Sparkline(id="replies-sparkline")

    def watch_auto_scroll(self, auto_scroll: bool) -> None:
        auto_scroll_label = self.auto_scroll_label
        auto_scroll_label.set_class(auto_scroll, "on")
        auto_scroll_label.set_class(not auto_scroll, "off")
        auto_scroll_label.update(
            "Auto Scroll [b]ON[/]" if auto_scroll else "Auto Scroll [b]OFF[/]"
        )

    @property
    def auto_scroll_label(self) -> Label:
        return self.query_one("#replies-auto-scroll-label", Label)


class Replies(VerticalScroll):
    """
    A widget for displaying replies from the server.
    """

    BINDING_GROUP_TITLE = "Incoming Messages"

    BINDINGS = [
        Binding(
            key="s",
            action="toggle_auto_scroll",
            description="Toggle Auto Scroll",
        ),
    ]

    number_of_replies: int = reactive(0)
    auto_scroll: bool = reactive(True)

    @dataclass
    class Incoming(Message):
        message: str
        timestamp: datetime.datetime

    def compose(self) -> ComposeResult:
        yield RepliesFooter(id="replies-footer").data_bind(Replies.auto_scroll)

    def on_mount(self) -> None:
        self.border_title = "Incoming"

    async def add_reply(self, message: Incoming) -> None:
        self.number_of_replies += 1
        await self.mount(Reply(message, self.number_of_replies))
        if self.auto_scroll:
            self.scroll_end()

    def watch_number_of_replies(self, number: int) -> None:
        self.border_subtitle = f"{number} messages received"

    def watch_auto_scroll(self, auto_scroll: bool) -> None:
        if auto_scroll:
            self.scroll_end()

    def action_toggle_auto_scroll(self) -> None:
        self.auto_scroll = not self.auto_scroll


class Reply(Vertical, can_focus=True, can_focus_children=False):
    BINDINGS = [
        Binding(key="enter", action="focus_text_area", description="Focus Text"),
        Binding(key="up", action="focus_previous", description="Up"),
        Binding(key="down", action="focus_next", description="Down"),
    ]

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

        yield ReplyTextArea(
            self.incoming.message, soft_wrap=False, id="reply-text-area"
        )

    def action_focus_text_area(self) -> None:
        self.query_one("#reply-text-area", ReadOnlyTextArea).focus()

    def action_focus_previous(self) -> None:
        self.screen.focus_previous(Reply)

    def action_focus_next(self) -> None:
        self.screen.focus_next(Reply)


class ReplyTextArea(ReadOnlyTextArea):
    BINDINGS = [
        Binding(key="escape,shift+tab", action="back", description="Back"),
    ]

    def action_back(self) -> None:
        self.parent.focus()

    def watch_has_focus(self, has_focus: bool) -> None:
        if not has_focus:
            self.selection = Selection.cursor((0, 0))
