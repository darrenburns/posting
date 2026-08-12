from __future__ import annotations

from dataclasses import dataclass

from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.screen import ModalScreen
from textual.validation import Length
from textual.widgets import Button, Footer, Label

from posting.widgets.input import PostingInput


@dataclass
class VariableEdit:
    name: str
    value: str


class VariableEditModal(ModalScreen[VariableEdit | None]):
    """Prompt for a variable name and value.

    Handles both editing an existing variable and adding a new one. When
    editing, the name stays fixed and you change only the value. Renaming in
    place would break every request that still referenced the old name, and
    nothing would report it until the next send.
    """

    CSS = """
    VariableEditModal {
        align: center middle;

        & .modal-body {
            width: 60;
            height: auto;
            padding: 1 2;
        }

        & Input {
            width: 1fr;
            margin-bottom: 1;
        }

        & Button {
            width: 1fr;
        }

        & .hint {
            color: $text-muted;
            margin-bottom: 1;
        }
    }
    """

    BINDINGS = [
        Binding("escape", "close_screen", "Cancel", show=False),
    ]

    def __init__(self, name: str = "", value: str = "", is_new: bool = False) -> None:
        super().__init__()
        self._name = name
        self._value = value
        self._is_new = is_new

    def compose(self) -> ComposeResult:
        with Vertical(classes="modal-body") as body:
            body.border_title = "Add variable" if self._is_new else "Edit variable"

            if self._is_new:
                yield Label("Name")
                yield PostingInput(
                    self._name,
                    placeholder="MY_VARIABLE",
                    validators=[
                        Length(minimum=1, failure_description="Name cannot be empty")
                    ],
                    id="name-input",
                )
            else:
                yield Label(f"Name [dim]{self._name}[/]")

            yield Label("Value")
            yield PostingInput(self._value, placeholder="Value", id="value-input")
            yield Label(
                "Saved for this session only. Environment files are not modified.",
                classes="hint",
            )
            yield Button.success("Save", id="save-button")

        yield Footer(show_command_palette=False)

    def on_mount(self) -> None:
        selector = "#name-input" if self._is_new else "#value-input"
        self.query_one(selector, PostingInput).focus()

    def action_close_screen(self) -> None:
        self.dismiss(None)

    @on(Button.Pressed, "#save-button")
    @on(PostingInput.Submitted)
    def save(self) -> None:
        if self._is_new:
            name = self.query_one("#name-input", PostingInput).value.strip()
            if not name:
                self.notify("Name cannot be empty", severity="error", timeout=3)
                return
        else:
            name = self._name
        value = self.query_one("#value-input", PostingInput).value
        self.dismiss(VariableEdit(name=name, value=value))
