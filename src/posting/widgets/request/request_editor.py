from textual import on, events
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.widgets import TabbedContent, Tabs, TabPane

from posting.widgets.request.header_editor import HeaderEditor
from posting.widgets.request.query_editor import QueryStringEditor
from posting.widgets.request.request_body import RequestBodyTextArea


class RequestEditorTabbedContent(TabbedContent):
    BINDINGS = [
        Binding("down,j", "app.focus_next", "Focus next", show=False),
        Binding("up,k", "app.focus_previous", "Focus previous", show=False),
    ]

    @on(events.DescendantFocus)
    def focus_within(self, event: events.DescendantFocus) -> None:
        print("focused")
        tabs = self.query_one(Tabs)
        if tabs.has_focus:
            self.refresh_bindings()

    def check_action(self, action: str, parameters: tuple[object, ...]) -> bool | None:
        """Check if an action may run."""
        print(action)
        if action == "next":
            return None
        if action == "previous":
            return None
        return True


class RequestEditor(Vertical):
    """
    The request editor.
    """

    DEFAULT_CSS = """\
    """

    def compose(self) -> ComposeResult:
        with Vertical() as vertical:
            vertical.border_title = "Request"
            with RequestEditorTabbedContent():
                with TabPane("Headers", id="headers-pane"):
                    yield HeaderEditor()
                with TabPane("Body", id="body-pane"):
                    yield RequestBodyTextArea(language="json")
                with TabPane("Parameters", id="parameters-pane"):
                    yield QueryStringEditor()

    def on_mount(self):
        self.border_title = "Request"
        self.add_class("section")
