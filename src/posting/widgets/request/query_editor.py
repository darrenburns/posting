from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.widgets import Input
from textual.widgets.data_table import CellDoesNotExist
from posting.collection import QueryParam

from posting.widgets.datatable import PostingDataTable
from posting.widgets.key_value import KeyValueEditor, KeyValueInput


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

    def action_remove_row(self) -> None:
        try:
            cursor_cell_key = self.coordinate_to_cell_key(self.cursor_coordinate)
            cursor_row_key, _ = cursor_cell_key
            self.remove_row(cursor_row_key)
        except CellDoesNotExist:
            pass

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
                Input(placeholder="Key"),
                Input(placeholder="Value"),
                button_label="Add parameter",
            ),
            empty_message="There are no parameters.",
        )
