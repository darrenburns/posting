import httpx
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.reactive import Reactive, reactive
from textual.widgets import TabbedContent, TabPane

from posting.widgets.response.response_body import ResponseTextArea
from posting.widgets.response.response_headers import ResponseHeadersTable


class ResponseTabbedContent(TabbedContent):
    BINDINGS = [
        Binding("down,j", "app.focus_next", "Focus next", show=False),
        Binding("up,k", "app.focus_previous", "Focus previous", show=False),
    ]


class ResponseArea(Vertical):
    """
    The response area.
    """

    DEFAULT_CSS = """\
    ResponseArea {
        border-subtitle-color: $text-muted;
        & ResponseTextArea.empty {
            display: none;
        }
        &.response-ready{
            & Crosshatch {
                display: none;
            }
        }
    }
    """
    response: Reactive[httpx.Response | None] = reactive(None)

    def on_mount(self) -> None:
        self.border_title = "Response"
        self.add_class("section")

    def compose(self) -> ComposeResult:
        with ResponseTabbedContent(disabled=self.response is None):
            with TabPane("Body", id="response-body-pane"):
                yield ResponseTextArea(language="json", read_only=True)
            with TabPane("Headers", id="response-headers-pane"):
                yield ResponseHeadersTable()

    def watch_response(self, response: httpx.Response | None) -> None:
        if response is None:
            return
        else:
            self.query_one(ResponseTabbedContent).disabled = False


        self.add_class("response-ready")

        # Update the body text area with the body content.
        response_text_area = self.response_text_area
        response_text_area.text = response.text
        response_text_area.focus()

        # Update the response headers table with the response headers.
        response_headers_table = self.response_headers_table
        response_headers_table.add_rows([(name, value) for name, value in response.headers.items()])

        if response.status_code < 300:
            style = "#ecfccb on #4d7c0f"
        elif response.status_code < 400:
            style = "black on yellow"
        else:
            style = "black on red"

        self.border_title = (
            f"Response [{style}] {response.status_code} {response.reason_phrase} [/]"
        )

        self.border_subtitle = f"{response.elapsed.total_seconds() * 1000:.2f} ms"

    @property
    def response_text_area(self) -> ResponseTextArea:
        return self.query_one(ResponseTextArea)

    @property
    def response_headers_table(self) -> ResponseHeadersTable:
        return self.query_one(ResponseHeadersTable)
