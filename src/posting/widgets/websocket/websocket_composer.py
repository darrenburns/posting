from datetime import datetime
import aiohttp
from textual import on, work, log
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.widget import Widget
from textual.widgets import TabPane, TabbedContent

from posting.widgets.text_area import PostingTextArea
from posting.widgets.websocket.message_composer import MessageEditor
from posting.widgets.websocket.replies import Replies
from posting.widgets.websocket.snippets import SnippetsLibrary


class WebsocketComposer(Vertical):
    BINDINGS = [
        Binding(
            key="ctrl+j,alt+enter",
            action="send_message",
            description="Send",
            tooltip="Send message or connect to websocket",
        ),
    ]

    def __init__(
        self,
        *children: Widget,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(
            *children, name=name, id=id, classes=classes, disabled=disabled
        )
        self.websocket: aiohttp.ClientWebSocketResponse | None = None
        self.session: aiohttp.ClientSession | None = None

    def compose(self) -> ComposeResult:
        self.border_title = "WebSocket"
        with TabbedContent(id="websocket-tabs", initial="message-composer"):
            with TabPane("Composer", id="message-composer"):
                yield MessageEditor()
            with TabPane("Snippets", id="snippets"):
                yield SnippetsLibrary()

    async def connect_websocket(self, url_value: str) -> None:
        self.session = aiohttp.ClientSession()
        try:
            self.websocket = await self.session.ws_connect(url_value)
        except Exception as e:
            log.error("Error connecting to websocket", e)
            self.notify(
                severity="error",
                title="Error connecting to websocket",
                message=str(e),
            )

        self.process_incoming_websocket_messages()

    async def send_message_or_connect(self, url_value: str) -> None:
        if self.websocket is None:
            await self.connect_websocket(url_value)
        else:
            await self.send_message(self.message_editor_content)

    async def action_send_message(self) -> None:
        await self.send_message(self.message_editor_content)

    async def send_message(self, message: str) -> None:
        if self.session.closed:
            print("Session closed - cannot send message")
            return

        print("Sending message", message)
        await self.websocket.send_str(message)

    @work(group="websocket-process-incoming", exclusive=True)
    async def process_incoming_websocket_messages(self) -> None:
        async for message in self.websocket:
            print(message)
            if message.type == aiohttp.WSMsgType.TEXT:
                self.post_message(
                    Replies.Incoming(message.data, timestamp=datetime.now())
                )
            elif message.type == aiohttp.WSMsgType.ERROR:
                log.error("Websocket error", message.data)

    @on(Replies.Incoming)
    async def on_incoming_message(self, message: Replies.Incoming) -> None:
        self.notify(title="Websocket", message=message.message)

    @property
    def message_editor_content(self) -> str:
        return self.query_one("#ws-message-text-area", PostingTextArea).text
