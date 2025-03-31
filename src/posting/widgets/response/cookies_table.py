from textual import on
from textual.app import ComposeResult
from textual.containers import Vertical
from textual.widgets import Label
from posting.widgets.center_middle import CenterMiddle
from posting.widgets.datatable import PostingDataTable


class CookiesSection(Vertical):
    def compose(self) -> ComposeResult:
        yield CenterMiddle(Label("No cookies"), id="empty-message")
        yield PostingDataTable(id="cookies-table")

    def on_mount(self) -> None:
        self.table.show_header = False
        self.table.cursor_type = "row"
        self.table.zebra_stripes = True
        self.table.fixed_columns = 1
        self.table.add_columns(*["Name", "Value"])

    @on(PostingDataTable.RowsRemoved)
    def rows_removed(self, event: PostingDataTable.RowsRemoved) -> None:
        rows = event.data_table.row_count
        self.set_class(rows == 0, "empty")

    @on(PostingDataTable.RowsAdded)
    def rows_added(self, event: PostingDataTable.RowsAdded) -> None:
        rows = event.data_table.row_count
        self.set_class(rows == 0, "empty")

    @property
    def table(self) -> PostingDataTable:
        return self.query_one("#cookies-table", PostingDataTable)
