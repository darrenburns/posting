import json
import httpx
from posting.config import SETTINGS

from posting.widgets.response.response_trace import ResponseTrace
from posting.widgets.response.script_output import ScriptOutput
from posting.widgets.tabbed_content import PostingTabbedContent
from posting.widgets.text_area import TextAreaFooter, TextEditor
from posting.widgets.response.cookies_table import CookiesTable
from posting.widgets.response.response_body import ResponseTextArea
from posting.widgets.response.response_headers import ResponseHeadersTable

from textual.app import ComposeResult
from textual.containers import Vertical
from textual.reactive import Reactive, reactive
from textual.widgets import TabPane
from textual.widgets._tabbed_content import ContentTabs


class ResponseTabbedContent(PostingTabbedContent):
    pass


class ResponseArea(Vertical):
    """
    The response area.
    """

    COMPONENT_CLASSES = {
        "border-title-status",
    }

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
        &.success .border-title-status {
            color: $text-success;
            background: $success-muted;
        }
        &.warning .border-title-status {
            color: $text-warning;
            background: $warning-muted;
        }
        &.error .border-title-status {
            color: $text-error;
            background: $error-muted;
        }
    }
    """
    response: Reactive[httpx.Response | None] = reactive(None)

    def on_mount(self) -> None:
        self.border_title = "Response"
        self._latest_response: httpx.Response | None = None
        self.add_class("section")
        self.app.theme_changed_signal.subscribe(self, self.on_theme_change)

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
            with TabPane("Scripts", id="response-scripts-pane"):
                yield ScriptOutput()
            with TabPane("Trace", id="response-trace-pane"):
                yield ResponseTrace()

    def on_theme_change(self, _) -> None:
        if self._latest_response:
            self.border_title = self._make_border_title(self._latest_response)

    def watch_response(self, response: httpx.Response | None) -> None:
        self._latest_response = response
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
                response_text = json.dumps(
                    json.loads(response_text), indent=2, ensure_ascii=False
                )
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

        self.remove_class("success", "warning", "error")
        if response.status_code < 300:
            self.add_class("success")
        elif response.status_code < 400:
            self.add_class("warning")
        else:
            self.add_class("error")

        self.border_title = self._make_border_title(response)

        settings = SETTINGS.get()
        if settings.response.show_size_and_time:
            self.border_subtitle = f"{human_readable_size(len(response.content))} in {response.elapsed.total_seconds() * 1000:.2f}[dim]ms[/]"

    def _make_border_title(self, response: httpx.Response) -> str:
        style = self.get_component_rich_style("border-title-status")
        return f"Response [{style}] {response.status_code} {response.reason_phrase} [/]"

    @property
    def text_editor(self) -> TextEditor:
        return self.query_one(TextEditor)

    @property
    def headers_table(self) -> ResponseHeadersTable:
        return self.query_one(ResponseHeadersTable)

    @property
    def cookies_table(self) -> CookiesTable:
        return self.query_one(CookiesTable)

    @property
    def tabbed_content(self) -> ResponseTabbedContent:
        return self.query_one(ResponseTabbedContent)

    @property
    def content_tabs(self) -> ContentTabs:
        return self.tabbed_content.query_one(ContentTabs)


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
