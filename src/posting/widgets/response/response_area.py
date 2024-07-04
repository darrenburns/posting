import json
import httpx
from posting.config import SETTINGS

from posting.widgets.response.response_trace import ResponseTrace
from posting.widgets.tabbed_content import PostingTabbedContent
from posting.widgets.text_area import TextAreaFooter, TextEditor
from posting.widgets.response.cookies_table import CookiesTable
from posting.widgets.response.response_body import ResponseTextArea
from posting.widgets.response.response_headers import ResponseHeadersTable

from textual.app import ComposeResult
from textual.containers import Vertical
from textual.reactive import Reactive, reactive
from textual.widgets import TabPane


class ResponseTabbedContent(PostingTabbedContent):
    pass


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
                text_area = ResponseTextArea(language="json")
                yield TextEditor(
                    text_area,
                    TextAreaFooter(text_area),
                )
            with TabPane("Headers", id="response-headers-pane"):
                yield ResponseHeadersTable()
            with TabPane("Cookies", id="response-cookies-pane"):
                yield CookiesTable()
            with TabPane("Trace", id="response-trace-pane"):
                yield ResponseTrace()

    def watch_response(self, response: httpx.Response | None) -> None:
        if response is None:
            return
        else:
            self.query_one(ResponseTabbedContent).disabled = False

        self.add_class("response-ready")

        content_type = response.headers.get("content-type")
        if content_type:
            language = content_type_to_language(content_type)
            self.text_editor.language = language

        # Update the body text area with the body content.
        response_text_area = self.text_editor.text_area
        response_text = response.text
        response_settings = SETTINGS.get().response
        if response_text_area.language == "json" and response_settings.prettify_json:
            try:
                response_text = json.dumps(json.loads(response_text), indent=2)
            except json.JSONDecodeError:
                pass

        response_text_area.text = response_text

        # Update the response headers table with the response headers.
        response_headers_table = self.headers_table
        response_headers_table.clear()
        response_headers_table.add_rows(
            [(name, value) for name, value in response.headers.items()]
        )

        # Update the response cookies table with the cookies from the response.
        cookies_table = self.cookies_table
        cookies_table.clear()
        cookies_table.add_rows(
            [(name, value) for name, value in response.cookies.items()]
        )

        if response.status_code < 300:
            style = "#ecfccb on #4d7c0f"
        elif response.status_code < 400:
            style = "black on yellow"
        else:
            style = "black on red"

        self.border_title = (
            f"Response [{style}] {response.status_code} {response.reason_phrase} [/]"
        )

        self.border_subtitle = f"{human_readable_size(len(response.content))} in {response.elapsed.total_seconds() * 1000:.2f}[dim]ms[/]"

    @property
    def text_editor(self) -> TextEditor:
        return self.query_one(TextEditor)

    @property
    def headers_table(self) -> ResponseHeadersTable:
        return self.query_one(ResponseHeadersTable)

    @property
    def cookies_table(self) -> CookiesTable:
        return self.query_one(CookiesTable)


def content_type_to_language(content_type: str) -> str | None:
    """Given the value of an HTTP content-type header, return the name
    of the language to use in the response body text area."""
    if content_type.startswith("application/json"):
        return "json"
    elif content_type.startswith("text/html") or content_type.startswith(
        "application/xml"
    ):
        return "html"
    elif content_type.startswith("text/css"):
        return "css"
    elif content_type.startswith("text/plain"):
        return None
    return "json"


def human_readable_size(size: float, decimal_places: int = 2) -> str:  # type: ignore
    for unit in ["B", "KB", "MB", "GB", "TB"]:
        if size < 1024:
            return f"{size:.{decimal_places}f}[dim]{unit}[/]"
        size /= 1024
