from rich.text import Text
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.content import Content
from textual.widgets import Input
from textual_autocomplete import DropdownItem, AutoComplete, TargetState
from posting.collection import Header
from posting.help_data import HelpData

from posting.widgets.datatable import PostingDataTable
from posting.request_headers import REQUEST_HEADERS
from posting.widgets.key_value import KeyValueEditor, KeyValueInput
from posting.widgets.input import PostingInput
from posting.widgets.variable_input import VariableInput


HEADER_SUGGESTIONS = {
    "content-type": [
        "application/",
        "application/json",
        "application/xml",
        "application/x-www-form-urlencoded",
        "multipart/",
        "multipart/form-data",
        "text/",
        "text/plain",
        "text/html",
        "text/css",
    ],
    "accept": [
        "application/",
        "application/json",
        "application/xml",
        "text/",
        "text/plain",
        "text/html",
        "text/css",
    ],
    "accept-encoding": [
        "gzip",
        "deflate",
        "br",
    ],
    "cache-control": [
        "no-cache",
        "no-store",
        "max-age=0",
        "must-revalidate",
        "proxy-revalidate",
    ],
}


class HeaderInput(PostingInput):
    help = HelpData(
        title="HTTP Header Input",
        description="""\
An input field for entering HTTP headers.
Press `up` and `down` to navigate the dropdown list when it's visible.
Press `enter` to insert the selected header.
Press `tab` to both insert *and* shift focus.
""",
    )

    BINDING_GROUP_TITLE = "HTTP Header Input"


class HeaderEditor(Vertical):
    BINDING_GROUP_TITLE = "HTTP Header Editor"

    def compose(self) -> ComposeResult:
        header_key_input = HeaderInput(placeholder="Name", id="header-key-input")
        yield KeyValueEditor(
            HeadersTable(),
            KeyValueInput(
                header_key_input,
                VariableInput(
                    placeholder="Value",
                    id="header-value-input",
                    candidates=self.get_header_value_candidates,
                ),
                button_label="Add",
            ),
            empty_message="No headers",
        )

    def on_mount(self):
        header_input = self.header_key_input
        items: list[DropdownItem] = []
        for header in REQUEST_HEADERS:
            style = "$text-warning" if header["experimental"] else ""
            content = Content.styled(header["name"], style=style)
            items.append(DropdownItem(main=content))

        self.screen.mount(
            AutoComplete(
                header_input,
                candidates=items,
                prevent_default_tab=False,
            )
        )

    def get_header_value_candidates(
        self, target_state: TargetState
    ) -> list[DropdownItem]:
        header_key = self.header_key_input.value.strip().lower()
        candidates = [
            DropdownItem(main=suggestion)
            for suggestion in HEADER_SUGGESTIONS.get(header_key, [])
        ]
        return candidates

    @property
    def header_key_input(self) -> HeaderInput:
        return self.query_one("#header-key-input", HeaderInput)


class HeadersTable(PostingDataTable):
    """
    The headers table.
    """

    help = HelpData(
        title="Headers Table",
        description="""\
A table of HTTP headers that will be sent with the request.
Press `backspace` to delete a header.
Press `space` to toggle a header on and off.
Posting will automatically attach a `User-Agent` header to outgoing requests in order to identify itself, and set the `Content-Type` depending on the content
in the body tab. Setting a header in this table will override the default value in these cases.
""",
    )

    BINDING_GROUP_TITLE = "Headers Table"

    BINDINGS = [
        Binding("backspace", action="remove_row", description="Remove header"),
        Binding("space", action="toggle_row", description="Toggle header"),
    ]

    def on_mount(self):
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.fixed_columns = 1
        self.row_disable = True
        self.add_columns(*["Header", "Value"])

    def watch_has_focus(self, value: bool) -> None:
        self._scroll_cursor_into_view()
        return super().watch_has_focus(value)

    def as_dict(self) -> dict[str, str]:
        headers: dict[str, str] = {}
        for row_index in range(self.row_count):
            row = self.get_row_at(row_index)
            if self.is_row_enabled_at(row_index):
                headers[row[0]] = row[1]
        return headers

    def to_model(self) -> list[Header]:
        headers: list[Header] = []
        for row_index in range(self.row_count):
            row = self.get_row_at(row_index)
            plain_row0 = row[0].plain if isinstance(row[0], Text) else row[0]
            plain_row1 = row[1].plain if isinstance(row[1], Text) else row[1]
            is_row_enabled = self.is_row_enabled_at(row_index)
            headers.append(
                Header(name=plain_row0, value=plain_row1, enabled=is_row_enabled)
            )
        return headers
