from typing import Iterable
from textual.app import ComposeResult
from textual.containers import Vertical
from textual.widgets import Input
from posting.collection import FormItem
from posting.widgets.datatable import PostingDataTable
from posting.widgets.key_value import KeyValueEditor, KeyValueInput


class FormTable(PostingDataTable):
    def on_mount(self):
        self.fixed_columns = 1
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.add_columns("Key", "Value")

    def to_model(self) -> list[FormItem]:
        form_data: list[FormItem] = []
        # TODO - handle enabled/disabled...
        for row_index in range(self.row_count):
            row = self.get_row_at(row_index)
            form_data.append(FormItem(name=row[0], value=row[1], enabled=True))
        return form_data


class FormEditor(Vertical):
    """An editor for form body data."""

    def compose(self) -> ComposeResult:
        yield KeyValueEditor(
            FormTable(),
            KeyValueInput(
                Input(placeholder="Key"),
                Input(placeholder="Value"),
            ),
            empty_message="There is no form data.",
        )

    def to_model(self) -> list[FormItem]:
        return self.query_one(FormTable).to_model()

    def replace_all_rows(self, rows: Iterable[tuple[str, str]]) -> None:
        self.query_one(FormTable).replace_all_rows(rows)
