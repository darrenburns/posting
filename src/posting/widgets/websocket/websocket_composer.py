from textual.app import ComposeResult
from textual.containers import Vertical
from textual.widgets import TabPane, TabbedContent

from posting.widgets.websocket.message_composer import MessageEditor
from posting.widgets.websocket.snippets import SnippetsLibrary


class WebsocketComposer(Vertical):
    def compose(self) -> ComposeResult:
        self.border_title = "Websocket"
        with TabbedContent(id="websocket-tabs", initial="message-composer"):
            with TabPane("Composer", id="message-composer"):
                yield MessageEditor()
            with TabPane("Snippets", id="snippets"):
                yield SnippetsLibrary()
