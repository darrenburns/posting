"""Asks which of a GraphQL document's operations to send."""

from __future__ import annotations

from typing import Sequence

from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.content import Content
from textual.screen import ModalScreen
from textual.widgets import Label, OptionList
from textual.widgets.option_list import Option

from posting.graphql.operations import Operation


class GraphQLOperationModal(ModalScreen[str | None]):
    """Lets the user pick the operation to run.

    Dismisses with the name of the chosen operation, or `None` if the user
    decided not to send the request after all.
    """

    BINDING_GROUP_TITLE = "GraphQL Operation Picker"

    DEFAULT_CSS = """
    GraphQLOperationModal {
        & .modal-body {
            max-width: 60;
        }

        & #operation-modal-description {
            width: 1fr;
            padding: 0 0 1 0;
            color: $text-muted;
        }

        & OptionList {
            border: none;
            padding: 0;
            background: transparent;

            &:focus {
                border: none;
            }

            & > .option-list--option {
                padding: 0 1;
            }
        }
    }
    """

    BINDINGS = [
        Binding("escape", "cancel", "Cancel", show=False),
        Binding("j", "cursor_down", "Down", show=False),
        Binding("k", "cursor_up", "Up", show=False),
        Binding("l", "select_highlighted", "Select", show=False),
    ]

    def __init__(self, operations: Sequence[Operation]) -> None:
        """
        Args:
            operations: The operations to choose between.
        """
        super().__init__()
        self.operations = operations

    def compose(self) -> ComposeResult:
        with Vertical(classes="modal-body") as container:
            container.border_title = "Send which operation?"
            yield Label(
                "This query defines more than one operation, so the request "
                "must say which one to run.",
                id="operation-modal-description",
            )
            yield OptionList(
                *(
                    Option(
                        Content.assemble((f"{operation.kind} ", "dim"), operation.name),
                        id=operation.name,
                    )
                    for operation in self.operations
                )
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

    def action_cancel(self) -> None:
        self.dismiss(None)
