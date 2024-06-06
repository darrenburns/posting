import httpx
from posting.widgets.text_area import ReadOnlyTextArea, TextAreaFooter
from posting.widgets.response.cookies_table import CookiesTable
from posting.widgets.response.response_body import ResponseTextArea
from posting.widgets.response.response_headers import ResponseHeadersTable

from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.reactive import Reactive, reactive
from textual.widgets import (
    TabbedContent,
    TabPane,
    TextArea,
)


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

        # Watch for changes from the configuration bar.
        body_config = self.body_config
        body_text_area = self.body_text_area

        def update_language(language: str):
            body_text_area.language = language

        def update_soft_wrap(soft_wrap: bool):
            body_text_area.soft_wrap = soft_wrap

        self.watch(body_config, "language", update_language)
        self.watch(body_config, "soft_wrap", update_soft_wrap)

    def compose(self) -> ComposeResult:
        with ResponseTabbedContent(disabled=self.response is None):
            with TabPane("Body", id="response-body-pane"):
                yield TextAreaFooter()
                yield ResponseTextArea(language="json")
            with TabPane("Headers", id="response-headers-pane"):
                yield ResponseHeadersTable()
            with TabPane("Cookies", id="response-cookies-pane"):
                yield CookiesTable()

    def watch_response(self, response: httpx.Response | None) -> None:
        if response is None:
            return
        else:
            self.query_one(ResponseTabbedContent).disabled = False

        self.add_class("response-ready")

        # Update the body text area with the body content.
        response_text_area = self.body_text_area
        response_text_area.text = response.text
        content_type = response.headers.get("content-type")
        if content_type:
            language = content_type_to_language(content_type)
            self.body_config.language = language
        response_text_area.focus()

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

    @on(TextArea.SelectionChanged, selector="ResponseTextArea")
    def update_selection(self, event: TextArea.SelectionChanged) -> None:
        self.body_config.selection = event.selection

    @on(ReadOnlyTextArea.VisualModeToggled, selector="ResponseTextArea")
    def update_visual_mode(self, event: ReadOnlyTextArea.VisualModeToggled) -> None:
        self.body_config.visual_mode = event.value

    @property
    def body_text_area(self) -> ResponseTextArea:
        return self.query_one(ResponseTextArea)

    @property
    def body_config(self) -> TextAreaFooter:
        return self.query_one(TextAreaFooter)

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
