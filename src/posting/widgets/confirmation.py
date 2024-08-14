"""A modal screen for confirming a destructive action."""

from textual.app import ComposeResult
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

    def __init__(
        self,
        message: str,
        confirm_text: str = "Yes \[y]",
        confirm_binding: str = "y",
        cancel_text: str = "No \[n]",
        cancel_binding: str = "n",
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

    def on_mount(self) -> None:
        self._bindings.bind(self.confirm_binding, "screen.dismiss(True)")
        self._bindings.bind(self.cancel_binding, "screen.dismiss(False)")
        self._bindings.bind("escape", "screen.dismiss(False)")

    def compose(self) -> ComposeResult:
        with Vertical(id="confirmation-screen", classes="modal-body") as container:
            container.border_title = "Confirm"
            yield Static(self.message)
            with Horizontal(id="confirmation-buttons"):
                yield Button(self.confirm_text, id="confirm-button")
                yield Button(self.cancel_text, id="cancel-button")
