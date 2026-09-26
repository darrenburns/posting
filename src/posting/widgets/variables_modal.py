from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.screen import ModalScreen
from textual.widgets import Footer, Input, Label

from posting.widgets.input import PostingInput
from posting.widgets.variables_editor import VariablesEditor


class VariablesModal(ModalScreen[None]):
    """Edit the variables available to requests without changing environment files."""

    CSS = """
    VariablesModal {
        align: center middle;

        & .modal-body {
            width: 80%;
            max-width: 100;
            height: 80%;
            max-height: 30;
            padding: 0 1;
        }

        & #variable-filter {
            border: none;
            padding: 0 1;
            height: 1;
            background: transparent;
        }

        & .hint {
            color: $text-muted;
            width: 1fr;
            height: auto;
            padding: 0 1;
            margin-bottom: 1;
        }

        & Footer {
            background: transparent;
        }
    }
    """

    BINDINGS = [
        Binding("escape", "close_or_defocus", "Close", show=False),
        Binding("slash", "focus_filter", "Filter"),
    ]

    def compose(self) -> ComposeResult:
        with Vertical(classes="modal-body") as container:
            container.border_title = "Variables"
            yield PostingInput(placeholder="Filter variables…", id="variable-filter")
            yield Label(
                "Session edits only. Environment files are unchanged.", classes="hint"
            )
            yield VariablesEditor()
        yield Footer(show_command_palette=False)

    def on_mount(self) -> None:
        self.query_one(VariablesEditor).focus_editor()

    @on(Input.Changed, "#variable-filter")
    def on_filter_changed(self, event: Input.Changed) -> None:
        editor = self.query_one(VariablesEditor)
        editor.filter_query = event.value
        editor.populate()

    def action_focus_filter(self) -> None:
        self.query_one(VariablesEditor).action_cancel_edit_row()
        self.query_one("#variable-filter", Input).focus()

    def action_close_or_defocus(self) -> None:
        editor = self.query_one(VariablesEditor)
        if isinstance(self.focused, Input) and editor.table.row_count:
            editor.table.focus()
        else:
            self.dismiss(None)
