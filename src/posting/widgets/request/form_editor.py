from textual import on
from textual.app import ComposeResult
from textual.containers import Vertical
from textual.widgets import Input
from posting.widgets.datatable import PostingDataTable
from posting.widgets.key_value import KeyValueEditor, KeyValueInput


class FormTable(PostingDataTable):
    def on_mount(self):
        self.fixed_columns = 1
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.add_columns("Key", "Value")


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

    @on(KeyValueInput.New)
    def on_new(self, event: KeyValueInput.New) -> None:
        print("new", event.key, event.value)
