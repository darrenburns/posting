"""A modal screen for confirming a destructive action."""

from textual.app import ComposeResult
from textual.containers import Vertical
from textual.screen import ModalScreen
from textual.widgets import Button, Static


class ConfirmationModal(ModalScreen[bool]):
    def __init__(
        self,
        message: str,
        confirm_text: str = "Yes [y]",
        confirm_binding: str = "y",
        cancel_text: str = "No [n]",
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

    def compose(self) -> ComposeResult:
        with Vertical(id="confirmation-screen"):
            yield Static(self.message)
            yield Button(self.confirm_text, id="yes-button")
            yield Button(self.cancel_text, id="no-button")
