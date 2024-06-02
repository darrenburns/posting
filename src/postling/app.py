from dataclasses import dataclass
from importlib.metadata import version
from itertools import cycle
from pathlib import Path
from typing import Iterable, Literal
import httpx
from rich.text import Text
from textual import events, on
from textual.css.query import NoMatches
from textual.design import ColorSystem
from textual.reactive import Reactive, reactive
from textual.events import Message
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.screen import ModalScreen, Screen
from textual.widgets import (
    Button,
    DataTable,
    Footer,
    Input,
    Label,
    OptionList,
    TabPane,
    TabbedContent,
    TextArea,
)
from textual.widgets._data_table import RowKey
from textual.widgets._tabbed_content import ContentTab
from textual.widgets.data_table import CellDoesNotExist
from textual.widgets.text_area import Location
from textual_autocomplete import AutoComplete, DropdownItem

from postling.commands import PostlingProvider
from postling.crosshatch import Crosshatch
from postling.highlight_url import URLHighlighter
from postling.messages import HttpResponseReceived
from postling.request_headers import REQUEST_HEADERS
from postling.text_area_theme import POSTLING_THEME


class AppHeader(Label):
    """The header of the app."""

    DEFAULT_CSS = """\
    AppHeader {
        color: $accent-lighten-2;
        padding: 1 3;
    }
    """


class AppBody(Vertical):
    """The body of the app."""

    DEFAULT_CSS = """\
    AppBody {
        padding: 1 2 0 2;
    }
    """


class MethodSelection(Label):
    """
    The label for the URL bar.
    """

    DEFAULT_CSS = """\
    MethodSelection {
        padding: 0 1;
        background: $accent-darken-1;
        color: $text;
        &:hover {
            background: $accent-darken-2;
        }
    }
    """

    @dataclass
    class Clicked(Message):
        """Posted when the method selection label is clicked."""

    @on(events.Click)
    def open_method_selection_popup(self, event: events.Click) -> None:
        self.post_message(MethodSelection.Clicked())

    def set_method(self, method: str) -> None:
        self.renderable = f"{method}"
        self.refresh(layout=True)


class MethodSelectionPopup(ModalScreen[str]):
    CSS = """\
    MethodSelectionPopup {
        & > Vertical {
            height: auto;
            width: auto;
            margin: 4 3;
            border-left: outer $primary-lighten-1;
            & > OptionList {
                background: transparent;
                width: auto;
                border: none;
            }
        }
    }"""

    BINDINGS = [
        Binding("escape", "app.pop_screen", "Dismiss"),
        Binding("g", "dismiss_with_http_method('GET')", "GET"),
        Binding("p", "dismiss_with_http_method('POST')", "POST"),
        Binding("a", "dismiss_with_http_method('PATCH')", "PATCH"),
        Binding("u", "dismiss_with_http_method('PUT')", "PUT"),
        Binding("d", "dismiss_with_http_method('DELETE')", "DELETE"),
        Binding("o", "dismiss_with_http_method('OPTIONS')", "OPTIONS"),
        Binding("h", "dismiss_with_http_method('HEAD')", "HEAD"),
    ]

    def compose(self) -> ComposeResult:
        with Vertical():
            yield OptionList("GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS")
        yield Footer()

    def on_click(self, event: events.Click) -> None:
        if self.get_widget_at(event.screen_x, event.screen_y)[0] is self:
            self.dismiss()

    @on(OptionList.OptionSelected)
    def return_selected_method(self, event: OptionList.OptionSelected) -> None:
        self.action_dismiss_with_http_method(event.option.prompt)

    def action_dismiss_with_http_method(self, method: str) -> None:
        self.dismiss(method)


class UrlInput(Input):
    """
    The URL input.
    """

    DEFAULT_CSS = """\
    UrlInput {
        border: none;
        width: 1fr;
        &:focus {
            border: none;
            padding: 0 1;
        }
    }
    """

    def on_mount(self):
        self.highlighter = URLHighlighter()


class SendRequestButton(Button, can_focus=False):
    """
    The button for sending the request.
    """

    DEFAULT_CSS = """\
    SendRequestButton {
        padding: 0 1;
        height: 1;
        min-width: 10;
        background: $success;
        color: $text;
        border: none;
        text-style: none;
        &:hover {
            text-style: b;
            padding: 0 1;
            border: none;
            background: $success;
        }
    }
    """


class UrlBar(Horizontal):
    """
    The URL bar.
    """

    DEFAULT_CSS = """\
    UrlBar {
        height: 1;
        padding: 0 3;
    }
    """

    def compose(self) -> ComposeResult:
        with Horizontal():
            yield MethodSelection("GET")
            yield UrlInput(
                "http://jsonplaceholder.typicode.com/posts",
                placeholder="Enter a URL...",
            )
            yield SendRequestButton("Send")


class RequestBodyTextArea(TextArea):
    """
    For editing request bodies.
    """

    DEFAULT_CSS = """\
    RequestBodyTextArea {
        border: none;

        &:focus {
            border: none;
        }

    }
    """

    OPENING_BRACKETS = {
        "(": ")",
        "[": "]",
        "{": "}",
    }

    CLOSING_BRACKETS = {v: k for k, v in OPENING_BRACKETS.items()}

    def on_mount(self):
        self.register_theme(POSTLING_THEME)
        self.theme = "postling"
        self.show_line_numbers = True
        self.tab_behavior = "indent"
        self.indent_width = 2
        self.set_class(len(self.text) == 0, "empty")

    @on(TextArea.Changed)
    def on_change(self, event: TextArea.Changed) -> None:
        self.set_class(len(self.text) == 0, "empty")

    def _on_key(self, event: events.Key) -> None:
        character = event.character

        if character in self.OPENING_BRACKETS:
            opener = character
            closer = self.OPENING_BRACKETS[opener]
            self.insert(f"{opener}{closer}")
            self.move_cursor_relative(columns=-1)
            event.prevent_default()
        elif character in self.CLOSING_BRACKETS:
            # If we're currently at a closing bracket and
            # we type the same closing bracket, move the cursor
            # instead of inserting a character.
            if self._matching_bracket_location:
                row, col = self.cursor_location
                line = self.document.get_line(row)
                if character == line[col]:
                    event.prevent_default()
                    self.move_cursor_relative(columns=1)
        elif event.key == "enter":
            row, column = self.cursor_location
            line = self.document.get_line(row)
            if not line:
                return

            column = min(column, len(line) - 1)
            character_locations = self._yield_character_locations_reverse(
                (row, max(0, column - 1))
            )
            rstrip_line = line[: column + 1].rstrip()
            anchor_char = rstrip_line[-1] if rstrip_line else None
            get_content_start_column = self.get_content_start_column
            get_column_width = self.get_column_width
            try:
                #
                for character, location in character_locations:
                    # Ignore whitespace
                    if character.isspace():
                        continue
                    elif character in self.OPENING_BRACKETS:
                        # We found an opening bracket on this line,
                        # so check the indentation of the line.
                        # The newly created line should have increased
                        # indentation.
                        content_start_col = get_content_start_column(line)
                        width = get_column_width(row, content_start_col)
                        width_to_indent = max(
                            width + self.indent_width, self.indent_width
                        )

                        target_location = row + 1, column + width_to_indent
                        insert_text = "\n" + " " * width_to_indent
                        print("Anchor char", anchor_char)
                        if anchor_char in self.CLOSING_BRACKETS:
                            # If there's a bracket under the cursor, we should
                            # ensure that gets indented too.
                            insert_text += "\n" + " " * content_start_col

                        self.insert(insert_text)
                        self.cursor_location = target_location
                        event.prevent_default()
                        break
                    else:
                        content_start_col = get_content_start_column(line)
                        width = get_column_width(row, content_start_col)
                        print("width", width)
                        self.insert("\n" + " " * width)
                        event.prevent_default()
                        break

                # If we make it to here, we didn't find an opening bracket
                # on the line, so just insert a newline and indent to the
                # same level as the previous line.

            except IndexError:
                return

        self._restart_blink()

    def get_content_start_column(self, line: str) -> int:
        content_start_col = 0
        for index, char in enumerate(line):
            if not char.isspace():
                content_start_col = index
                break
        return content_start_col

    def _yield_character_locations_reverse(
        self, start: Location
    ) -> Iterable[tuple[str, Location]]:
        row, column = start
        document = self.document
        line_count = document.line_count

        while line_count > row >= 0:
            line = document[row]
            if column == -1:
                column = len(line) - 1
            while column >= 0:
                yield line[column], (row, column)
                column -= 1
            row -= 1


class ResponseTextArea(TextArea):
    """
    For displaying responses.
    """

    DEFAULT_CSS = """\
    ResponseTextArea {
        border: none;
        padding: 0;
        &:focus {
            border: none;
            padding: 0;
        }
    }
    """

    def on_mount(self):
        self.register_theme(POSTLING_THEME)
        self.theme = "postling"
        empty = len(self.text) == 0
        self.set_class(empty, "empty")
        self.show_line_numbers = not empty

    @on(TextArea.Changed)
    def on_change(self, event: TextArea.Changed) -> None:
        empty = len(self.text) == 0
        self.set_class(empty, "empty")
        self.show_line_numbers = not empty


class HeadersTable(DataTable[str]):
    """
    The headers table.
    """

    DEFAULT_CSS = """\
    HeadersTable {
        height: auto;
        width: 1fr;
        border-left: inner $accent 0%;
        margin-right: 1;

        &:focus {
            width: 1fr;
            border-left: inner $accent;
        }
    }
    """

    BINDINGS = [
        Binding("backspace", action="remove_header", description="Remove header"),
    ]

    @dataclass
    class Changed(Message):
        data_table: DataTable[str]

    def on_mount(self):
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.add_columns(*["Header", "Value"])
        self.add_row("Content-Type", "application/json")
        self.add_row("Some-Header", "Some value")

    def watch_has_focus(self, value: bool) -> None:
        self._scroll_cursor_into_view()
        return super().watch_has_focus(value)

    def as_dict(self) -> dict[str, str]:
        headers: dict[str, str] = {}
        for row_index in range(self.row_count):
            row = self.get_row_at(row_index)
            headers[row[0]] = row[1]
        return headers

    def add_row(
        self,
        *cells: str,
        height: int | None = 1,
        key: str | None = None,
        label: str | Text | None = None,
    ) -> RowKey:
        # TODO - this event was not bubbling up to the screen,
        # and I have no clue why. So I'll just post it directly
        # to the screen for now.
        self.screen.post_message(HeadersTable.Changed(self))
        return super().add_row(*cells, height=height, key=key, label=label)

    def remove_row(self, row_key: RowKey | str) -> None:
        self.screen.post_message(HeadersTable.Changed(self))
        return super().remove_row(row_key)

    def action_remove_header(self) -> None:
        try:
            cursor_cell_key = self.coordinate_to_cell_key(self.cursor_coordinate)
            cursor_row_key, _ = cursor_cell_key
            self.remove_row(cursor_row_key)
        except CellDoesNotExist:
            pass


class HeaderEditor(Vertical):
    DEFAULT_CSS = """\
    #header-inputs {
        height: 1;
        dock: bottom;
        & > Input {
            border: none;
            width: 1fr;
            margin-left: 1;
        }

        #add-header-button {
            background: $success;
            color: $text;
            text-style: none;
            width: 10;
            margin: 0 1;
            &:hover {
                text-style: b;
                padding: 0 1;
                border: none;
                background: $success-darken-1;
            }
        
        }
    }
    """

    def compose(self) -> ComposeResult:
        with Horizontal(id="header-inputs"):
            yield Input(placeholder="Header name", id="header-key-input")
            yield Input(placeholder="Header value", id="header-value-input")
            add_header_button = Button(
                "Add header", disabled=True, id="add-header-button"
            )
            add_header_button.can_focus = False
            yield add_header_button
        yield HeadersTable()

    def on_mount(self):
        header_input = self.query_one("#header-key-input", Input)
        items: list[DropdownItem] = []
        for header in REQUEST_HEADERS:
            style = "yellow" if header["experimental"] else ""
            items.append(DropdownItem(main=Text(header["name"], style=style)))

        self.screen.mount(
            AutoComplete(
                header_input,
                items=items,
                prevent_default_tab=False,
                prevent_default_enter=False,
            )
        )

    @on(Input.Changed, selector="#header-key-input")
    @on(Input.Changed, selector="#header-value-input")
    def determine_button_enabled(self) -> None:
        # An HTTP header must have a key, but not necessarily a value.
        key_input = self.query_one("#header-key-input", Input)
        button = self.query_one("#add-header-button", Button)
        button.disabled = not key_input.value

    @on(Input.Submitted, selector="#header-key-input")
    @on(Input.Submitted, selector="#header-value-input")
    @on(Button.Pressed, selector="#add-header-button")
    def add_header(self, event: Input.Submitted | Button.Pressed) -> None:
        key_input = self.query_one("#header-key-input", Input)
        value_input = self.query_one("#header-value-input", Input)

        key = key_input.value
        value = value_input.value
        table = self.query_one(HeadersTable)

        def add_header() -> None:
            table.add_row(key, value)
            key_input.clear()
            value_input.clear()
            key_input.focus()
            table.move_cursor(row=table.row_count - 1)

        if key and value:
            add_header()
        elif key and not value:
            # This is a technically valid, but unlikely.
            if isinstance(event, Input.Submitted):
                input_id = event.input.id
                if input_id == "header-key-input":
                    value_input.focus()
                elif input_id == "header-value-input":
                    add_header()
            else:
                add_header()
        elif value and not key:
            key_input.focus()
        else:
            # Case where both are empty - do nothing.
            pass


class ParamsTable(DataTable[str]):
    """
    The parameters table.
    """

    DEFAULT_CSS = """\
    ParamsTable {
        height: auto;
        width: 1fr;
        border-left: inner $accent 0%;
        margin-right: 1;

        &:focus {
            width: 1fr;
            border-left: inner $accent;
        }
    }
    """

    BINDINGS = [
        Binding("backspace", action="remove_row", description="Remove row"),
    ]

    @dataclass
    class Changed(Message):
        data_table: DataTable[str]

    def on_mount(self):
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.add_columns(*["Key", "Value"])

    def watch_has_focus(self, value: bool) -> None:
        self._scroll_cursor_into_view()
        return super().watch_has_focus(value)

    def add_row(
        self,
        *cells: str,
        height: int | None = 1,
        key: str | None = None,
        label: str | Text | None = None,
    ) -> RowKey:
        # TODO - this event was not bubbling up to the screen,
        # and I have no clue why. So I'll just post it directly
        # to the screen for now.
        self.screen.post_message(ParamsTable.Changed(self))
        return super().add_row(*cells, height=height, key=key, label=label)

    def remove_row(self, row_key: RowKey | str) -> None:
        self.screen.post_message(ParamsTable.Changed(self))
        return super().remove_row(row_key)

    def action_remove_row(self) -> None:
        try:
            cursor_cell_key = self.coordinate_to_cell_key(self.cursor_coordinate)
            cursor_row_key, _ = cursor_cell_key
            self.remove_row(cursor_row_key)
        except CellDoesNotExist:
            pass

    def as_dict(self) -> dict[str, str]:
        params: dict[str, str] = {}
        for row_index in range(self.row_count):
            row = self.get_row_at(row_index)
            params[row[0]] = row[1]
        return params


class QueryStringEditor(Vertical):
    """
    The query string editor.
    """

    # The CSS is very similar to the header editor
    DEFAULT_CSS = """\
    QueryStringEditor {
        & #param-inputs {
            height: 1;
            dock: bottom;
            & > Input {
                border: none;
                width: 1fr;
                margin-left: 1;

                &:focus {
                    background: $surface-lighten-1;
                }
            }

            #add-param-button {
                background: $success;
                color: $text;
                text-style: none;
                width: 10;
                margin: 0 1;
                &:hover {
                    text-style: b;
                    padding: 0 1;
                    border: none;
                    background: $success-darken-1;
                }
            }
        }
    }
    """

    def compose(self) -> ComposeResult:
        with Horizontal(id="param-inputs"):
            yield Input(placeholder="Key", id="param-key-input")
            yield Input(placeholder="Value", id="param-value-input")
            yield Button(label="Add param", disabled=True, id="add-param-button")

        yield ParamsTable()

    @on(Input.Changed, selector="#param-key-input")
    @on(Input.Changed, selector="#param-value-input")
    def determine_button_enabled(self) -> None:
        key_input = self.query_one("#param-key-input", Input)
        button = self.query_one("#add-param-button", Button)
        button.disabled = not key_input.value

    @on(Input.Submitted, selector="#param-key-input")
    @on(Input.Submitted, selector="#param-value-input")
    @on(Button.Pressed, selector="#add-param-button")
    def add_param(self, event: Input.Submitted | Button.Pressed) -> None:
        key_input = self.query_one("#param-key-input", Input)
        value_input = self.query_one("#param-value-input", Input)
        key = key_input.value
        value = value_input.value
        table = self.query_one(ParamsTable)

        def add_param() -> None:
            table.add_row(key, value)
            key_input.clear()
            value_input.clear()
            key_input.focus()
            table.move_cursor(row=table.row_count - 1)

        if key and value:
            add_param()
        elif key and not value:
            # This is a technically valid, but unlikely.
            if isinstance(event, Input.Submitted):
                input_id = event.input.id
                if input_id == "param-key-input":
                    value_input.focus()
                elif input_id == "param-value-input":
                    add_param()
        elif value and not key:
            key_input.focus()
        else:
            # Case where both are empty - do nothing.
            pass


class RequestEditor(Vertical):
    """
    The request editor.
    """

    DEFAULT_CSS = """\
    """

    def compose(self) -> ComposeResult:
        with Vertical() as vertical:
            vertical.border_title = "Request"
            with TabbedContent():
                with TabPane("Headers", id="headers-pane"):
                    yield HeaderEditor()
                with TabPane("Body", id="body-pane"):
                    yield RequestBodyTextArea(language="json")
                with TabPane("Parameters", id="parameters-pane"):
                    yield QueryStringEditor()

    def on_mount(self):
        self.border_title = "Request"
        self.add_class("section")


class ResponseArea(Vertical):
    """
    The response area.
    """

    DEFAULT_CSS = """\
    ResponseArea {
        border-subtitle-color: $text-muted;
    }
    """
    response: Reactive[httpx.Response | None] = reactive(None)

    def on_mount(self) -> None:
        self.border_title = "Response"
        self.add_class("section")

    def compose(self) -> ComposeResult:
        yield ResponseTextArea(language="json", read_only=True)

    def watch_response(self, response: httpx.Response | None) -> None:
        if response is None:
            return

        response_text_area = self.response_text_area
        response_text_area.text = response.text
        response_text_area.focus()

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


class MainScreen(Screen[None]):
    BINDINGS = [
        Binding("ctrl+j", "send_request", "Send request"),
        Binding("ctrl+t", "change_method", "Change method"),
        Binding("ctrl+i", "focus_headers", "Headers"),
        # Binding("ctrl+n", "tree", "DEBUG Show tree"),
    ]

    selected_method = reactive("GET")
    layout: Reactive[Literal["horizontal", "vertical"]] = reactive("vertical")

    def compose(self) -> ComposeResult:
        yield AppHeader(f"[i]Postling[/] [white dim]{version('postling')}[/]")
        yield UrlBar()
        with AppBody():
            yield RequestEditor()
            yield ResponseArea()
        yield Footer()

    @on(Button.Pressed, selector="SendRequestButton")
    @on(Input.Submitted, selector="UrlInput")
    async def send_request(self) -> None:
        try:
            async with httpx.AsyncClient() as client:
                # TODO - update the request object here.
                # TODO - think about whether we store a single request instance or create a new one each time.
                request = self.build_httpx_request()
                print("-- sending request --")
                print(request)
                print(request.headers)
                response = await client.send(request=request)
                self.post_message(HttpResponseReceived(response))

        except Exception:
            pass

    @on(HttpResponseReceived)
    def on_response_received(self, event: HttpResponseReceived) -> None:
        # TODO - call method on the response section
        self.response_area.response = event.response

    async def action_send_request(self) -> None:
        await self.send_request()

    def action_change_method(self) -> None:
        self.method_selection()

    def action_focus_headers(self) -> None:
        self.headers_table.focus()

    def watch_layout(self, layout: Literal["horizontal", "vertical"]) -> None:
        app_body = self.app_body
        app_body.styles.layout = layout

    # def action_tree(self) -> None:
    #     from textual import log

    #     log.info(self.app.tree)
    #     log(self.app.get_css_variables())
    #     self.app.next_theme()

    @on(TextArea.Changed, selector="RequestBodyTextArea")
    def on_request_body_change(self, event: TextArea.Changed) -> None:
        body_tab = self.query_one("#--content-tab-body-pane", ContentTab)
        if event.text_area.text:
            body_tab.update("Body[cyan b]•[/]")
        else:
            body_tab.update("Body")

    @on(HeadersTable.Changed)
    def on_content_changed(self, event: HeadersTable.Changed) -> None:
        print("on_content_changed")
        headers_tab = self.query_one("#--content-tab-headers-pane", ContentTab)
        print("event.data_table.row_count", event.data_table.row_count)
        if event.data_table.row_count:
            headers_tab.update("Headers[cyan b]•[/]")
        else:
            headers_tab.update("Headers")

    @on(ParamsTable.Changed)
    def on_params_changed(self, event: ParamsTable.Changed) -> None:
        params_tab = self.query_one("#--content-tab-parameters-pane", ContentTab)
        if event.data_table.row_count:
            params_tab.update("Parameters[cyan b]•[/]")
        else:
            params_tab.update("Parameters")

    @on(MethodSelection.Clicked)
    def method_selection(self) -> None:
        def set_method(method: str) -> None:
            self.selected_method = method

        self.app.push_screen(MethodSelectionPopup(), callback=set_method)

    def build_httpx_request(self) -> httpx.Request:
        return httpx.Request(
            method=self.selected_method,
            url=self.url_input.value,
            params=self.params_table.as_dict(),
            content=self.request_body_text_area.text,
            headers=self.headers_table.as_dict(),
        )

    @property
    def url_input(self) -> UrlInput:
        return self.query_one(UrlInput)

    @property
    def response_area(self) -> ResponseArea:
        return self.query_one(ResponseArea)

    @property
    def request_body_text_area(self) -> RequestBodyTextArea:
        return self.query_one(RequestBodyTextArea)

    @property
    def headers_table(self) -> HeadersTable:
        return self.query_one(HeadersTable)

    @property
    def params_table(self) -> ParamsTable:
        return self.query_one(ParamsTable)

    @property
    def app_body(self) -> AppBody:
        return self.query_one(AppBody)

    def watch_selected_method(self, value: str) -> None:
        self.query_one(MethodSelection).set_method(value)


class Postling(App[None]):
    COMMANDS = {PostlingProvider}
    CSS_PATH = Path(__file__).parent / "postling.scss"
    BINDINGS = [
        Binding(
            "ctrl+p",
            "command_palette",
            description="Commands",
            show=True,
        ),
    ]

    themes: dict[str, ColorSystem] = {
        "textual": ColorSystem(
            primary="#004578",
            secondary="#ffa62b",
            warning="#ffa62b",
            error="#ba3c5b",
            success="#4EBF71",
            accent="#0178D4",
            dark=True,
        ),
        "sunset": ColorSystem(
            primary="#ff4500",
            secondary="#ff8c00",
            warning="#ff6347",
            error="#b22222",
            success="#32cd32",
            accent="#ffd700",
            dark=True,
        ),
        "ocean": ColorSystem(
            primary="#1e90ff",
            secondary="#00ced1",
            warning="#ffa07a",
            error="#ff4500",
            success="#20b2aa",
            accent="#4682b4",
            dark=True,
        ),
    }

    theme: Reactive[str | None] = reactive(None)

    def __init__(self):
        super().__init__()
        self.theme = "textual"
        self.themes_cycle = cycle(self.themes.items())

    def get_default_screen(self) -> MainScreen:
        self.main_screen = MainScreen()
        return self.main_screen

    def get_css_variables(self) -> dict[str, str]:
        if self.theme:
            system = self.themes.get(self.theme)
            if system:
                theme = system.generate()
            else:
                theme = {}
        else:
            theme = {}
        return {**super().get_css_variables(), **theme}

    def command_layout(self, layout: Literal["vertical", "horizontal"]) -> None:
        self.main_screen.layout = layout

    def command_theme(self, theme: str) -> None:
        self.theme = theme
        self.refresh_css()
        self.notify(f"Theme is now [b]{theme}[/].", title="Theme updated", timeout=2.5)


app = Postling()
if __name__ == "__main__":
    app.run()
