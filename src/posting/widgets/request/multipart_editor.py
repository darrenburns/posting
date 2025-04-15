from typing import Iterable
from rich.text import Text
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from posting.collection import MultipartItem
from posting.widgets.datatable import PostingDataTable
from posting.widgets.key_value import KeyValueEditor, KeyValueInput
from posting.widgets.variable_input import VariableInput


class MultipartTable(PostingDataTable):
    BINDINGS = [
        Binding("backspace", action="remove_row", description="Remove row"),
        Binding("space", action="toggle_row", description="Toggle row"),
    ]

    def on_mount(self):
        self.fixed_columns = 1
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.row_disable = True
        self.add_columns("Key", "Value")

    def to_model(self) -> list[MultipartItem]:
        multipart_data: list[MultipartItem] = []
        for row_index in range(self.row_count):
            row = self.get_row_at(row_index)
            multipart_data.append(
                MultipartItem(
                    name=row[0].plain if isinstance(row[0], Text) else row[0],
                    value=row[1].plain if isinstance(row[1], Text) else row[1],
                    enabled=self.is_row_enabled_at(row_index),
                )
            )
        return multipart_data


class MultipartEditor(Vertical):
    """An editor for multipart body data."""

    def compose(self) -> ComposeResult:
        yield KeyValueEditor(
            MultipartTable(),
            KeyValueInput(
                VariableInput(placeholder="Key"),
                VariableInput(placeholder="Value"),
            ),
            empty_message="There is no multipart data.",
        )

    def to_model(self) -> list[MultipartItem]:
        return self.query_one(MultipartTable).to_model()

    def replace_all_rows(
        self, rows: Iterable[Iterable[str]], enableStates: Iterable[bool] | None = None
    ) -> None:
        self.query_one(MultipartTable).replace_all_rows(rows, enableStates)
