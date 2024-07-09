from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.widgets.data_table import CellDoesNotExist
from posting.collection import QueryParam

from posting.widgets.datatable import PostingDataTable
from posting.widgets.key_value import KeyValueEditor, KeyValueInput
from posting.widgets.variable_input import VariableInput


class ParamsTable(PostingDataTable):
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

    def on_mount(self):
        self.fixed_columns = 1
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.add_columns("Key", "Value")

    def watch_has_focus(self, value: bool) -> None:
        self._scroll_cursor_into_view()
        return super().watch_has_focus(value)

    def to_model(self) -> list[QueryParam]:
        params: list[QueryParam] = []
        # TODO - handle enabled/disabled...
        for row_index in range(self.row_count):
            row = self.get_row_at(row_index)
            params.append(QueryParam(name=row[0], value=row[1], enabled=True))
        return params


class QueryStringEditor(Vertical):
    """
    The query string editor.
    """

    def compose(self) -> ComposeResult:
        yield KeyValueEditor(
            ParamsTable(),
            KeyValueInput(
                VariableInput(placeholder="Key"),
                VariableInput(placeholder="Value"),
                button_label="Add parameter",
            ),
            empty_message="There are no parameters.",
        )
