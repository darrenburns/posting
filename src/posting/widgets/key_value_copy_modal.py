from typing import Literal

from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.screen import ModalScreen
from textual.widgets import OptionList
from textual.widgets.option_list import Option
from textual import on


CopyChoice = Literal["name", "value", "both"] | None


class KeyValueCopyModal(ModalScreen[CopyChoice]):
    """Modal for selecting what to copy from a key-value row."""

    DEFAULT_CSS = """
    KeyValueCopyModal {
        align: center middle;

        & .modal-body {
            width: auto;
            max-width: 30;
            padding: 0;
        }

        & OptionList {
            width: auto;
            border: none;
            padding: 0;
            background: transparent;

            &:focus {
                border: none;
            }

            & > .option-list--option {
                padding: 0 2;
            }
        }
    }
    """

    BINDINGS = [
        Binding("escape", "dismiss(None)", "Cancel", show=False),
        Binding("j", "cursor_down", "Down", show=False),
        Binding("k", "cursor_up", "Up", show=False),
        Binding("l", "select_highlighted", "Select", show=False),
        Binding("n", "select('name')", "Copy name", show=False),
        Binding("v", "select('value')", "Copy value", show=False),
        Binding("b", "select('both')", "Copy both", show=False),
    ]

    def compose(self) -> ComposeResult:
        with Vertical(classes="modal-body") as container:
            container.border_title = "Copy"
            yield OptionList(
                Option(r"Copy name    [dim]\[n][/]", id="name"),
                Option(r"Copy value   [dim]\[v][/]", id="value"),
                Option(r"Copy both    [dim]\[b][/]", id="both"),
            )

    def on_mount(self) -> None:
        self.query_one(OptionList).focus()

    @on(OptionList.OptionSelected)
    def on_option_selected(self, event: OptionList.OptionSelected) -> None:
        self.dismiss(event.option.id)

    def action_cursor_down(self) -> None:
        self.query_one(OptionList).action_cursor_down()

    def action_cursor_up(self) -> None:
        self.query_one(OptionList).action_cursor_up()

    def action_select_highlighted(self) -> None:
        self.query_one(OptionList).action_select()

    def action_select(self, choice: CopyChoice) -> None:
        self.dismiss(choice)
