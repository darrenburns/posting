"""A modal screen for confirming a destructive action."""

from typing import Literal
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.screen import ModalScreen
from textual.widgets import Button, Static


class ConfirmationModal(ModalScreen[bool]):
    DEFAULT_CSS = """
    ConfirmationModal {
        align: center middle;
        height: auto;
        & #confirmation-buttons {
            margin-top: 1;
            width: 100%;
            height: 1;
            align: center middle;

            & > Button {
                width: 1fr;
            }
        }
    }
    """

    BINDINGS = [
        Binding(
            "left,right,up,down,h,j,k,l",
            "move_focus",
            "Navigate",
            show=False,
        )
    ]

    def __init__(
        self,
        message: str,
        confirm_text: str = "Yes \\[y]",
        confirm_binding: str = "y",
        cancel_text: str = "No \\[n]",
        cancel_binding: str = "n",
        auto_focus: Literal["confirm", "cancel"] | None = "confirm",
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
    ) -> None:
        super().__init__(name=name, id=id, classes=classes)
        self.message = message
        self.confirm_text = confirm_text
        self.confirm_binding = confirm_binding
        self.cancel_text = cancel_text
        self.cancel_binding = cancel_binding
        self.auto_focus = auto_focus

    def on_mount(self) -> None:
        self._bindings.bind(self.confirm_binding, "screen.dismiss(True)")
        self._bindings.bind(self.cancel_binding, "screen.dismiss(False)")
        self._bindings.bind("escape", "screen.dismiss(False)")
        if self.auto_focus is not None:
            self.query_one(f"#{self.auto_focus}-button").focus()

    def compose(self) -> ComposeResult:
        with Vertical(id="confirmation-screen", classes="modal-body") as container:
            container.border_title = "Confirm"
            yield Static(self.message)
            with Horizontal(id="confirmation-buttons"):
                yield Button(self.confirm_text, id="confirm-button")
                yield Button(self.cancel_text, id="cancel-button")

    @on(Button.Pressed, "#confirm-button")
    def confirm(self) -> None:
        self.dismiss(True)

    @on(Button.Pressed, "#cancel-button")
    def cancel(self) -> None:
        self.dismiss(False)

    def action_move_focus(self) -> None:
        # It's enough to just call focus_next here as there are only two buttons.
        self.screen.focus_next()
