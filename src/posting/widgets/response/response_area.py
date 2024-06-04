import httpx
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical, Horizontal
from textual.reactive import Reactive, reactive
from textual.widgets import TabbedContent, TabPane, Select, Checkbox, Switch, Label
from textual.widgets._toggle_button import ToggleButton

from posting.widgets.response.cookies_table import CookiesTable
from posting.widgets.response.response_body import ResponseTextArea
from posting.widgets.response.response_headers import ResponseHeadersTable


class ResponseTabbedContent(TabbedContent):
    BINDINGS = [
        Binding("down,j", "app.focus_next", "Focus next", show=False),
        Binding("up,k", "app.focus_previous", "Focus previous", show=False),
    ]


class ResponseBodyConfig(Horizontal):
    """The bar that appears above the response body, allowing
    you to customise the syntax highlighting, wrapping, line numbers,
    etc.
    """

    DEFAULT_CSS = """\
    ResponseBodyConfig {
        dock: bottom;
        height: 1;
        width: 1fr;
        background: $primary;
        
        &:disabled {
            background: transparent;
        }
        
        & Select {
            width: 8;
            & SelectCurrent {
                width: 8;
            }
            & SelectOverlay {
                width: 16;
            }
        }
        
        & Checkbox {
            margin-left: 1;
        }
    }
    """

    language = reactive("json", init=False)

    def watch_language(self, language: str) -> None:
        print("chose language = {language}".format(language=language))
        self.query_one(Select).value = language

    def compose(self) -> ComposeResult:
        with Horizontal(classes="dock-left w-auto"):
            yield Select(prompt="Content type", value=self.language, allow_blank=False,
                         options=[("JSON", "json"), ("HTML", "html")],
                         id="response-content-type-select")
            yield Checkbox(label="Wrap", value=True, button_first=False, id="response-wrap-checkbox")


def content_type_to_language(content_type: str) -> str | None:
    """Given the value of an HTTP content-type header, return the name
    of the language to use in the response body text area."""
    if content_type.startswith("application/json"):
        return "json"
    elif content_type.startswith("text/html") or content_type.startswith("application/xml"):
        return "html"
    elif content_type.startswith("text/css"):
        return "css"
    elif content_type.startswith("plain"):
        return None
    return "json"


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
                yield ResponseBodyConfig()
                yield ResponseTextArea(language="json", read_only=True)
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
            self.body_text_area.language = language
            self.body_config.language = language
        response_text_area.focus()

        # Update the response headers table with the response headers.
        response_headers_table = self.headers_table
        response_headers_table.clear()
        response_headers_table.add_rows([(name, value) for name, value in response.headers.items()])

        # Update the response cookies table with the cookies from the response.
        cookies_table = self.cookies_table
        cookies_table.clear()
        cookies_table.add_rows([(name, value) for name, value in response.cookies.items()])

        print(response.cookies)

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

    @on(Checkbox.Changed, selector="#response-wrap-checkbox")
    def wrap_toggled(self, event: Checkbox.Changed) -> None:
        self.body_text_area.soft_wrap = event.value

    @on(Select.Changed, selector="#response-content-type-select")
    def content_type_changed(self, event: Select.Changed) -> None:
        self.body_config.language = event.value
        self.body_text_area.language = event.value

    @property
    def body_text_area(self) -> ResponseTextArea:
        return self.query_one(ResponseTextArea)

    @property
    def body_config(self) -> ResponseBodyConfig:
        return self.query_one(ResponseBodyConfig)

    @property
    def headers_table(self) -> ResponseHeadersTable:
        return self.query_one(ResponseHeadersTable)

    @property
    def cookies_table(self) -> CookiesTable:
        return self.query_one(CookiesTable)
