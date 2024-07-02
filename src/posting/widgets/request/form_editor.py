from textual.app import ComposeResult
from textual.containers import Vertical
from textual.widgets import Input
from posting.widgets.datatable import PostingDataTable
from posting.widgets.key_value import KeyValueEditor, KeyValueInput


class FormTable(PostingDataTable):
    pass


class FormEditor(Vertical):
    """An editor for form body data."""

    def compose(self) -> ComposeResult:
        yield KeyValueEditor(
            PostingDataTable(),
            KeyValueInput(
                Input(placeholder="Key"),
                Input(placeholder="Value"),
            ),
            empty_message="There is no form data.",
        )
