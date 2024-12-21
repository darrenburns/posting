from textual import on
from textual.app import ComposeResult
from textual.containers import Vertical

from posting.widgets.websocket.replies import Replies
from posting.widgets.websocket.websocket_composer import WebsocketComposer


class WebSocketContainer(Vertical):
    def compose(self) -> ComposeResult:
        yield WebsocketComposer(classes="section")
        yield Replies(classes="section")

    @on(Replies.Incoming)
    def on_incoming(self, event: Replies.Incoming) -> None:
        self.replies.add_reply(event)

    @property
    def composer(self) -> WebsocketComposer:
        return self.query_one(WebsocketComposer)

    @property
    def replies(self) -> Replies:
        return self.query_one(Replies)
