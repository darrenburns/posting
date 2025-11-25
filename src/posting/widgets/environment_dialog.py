from textual.app import ComposeResult
from textual.screen import ModalScreen
from textual.widgets import DataTable, Button
from textual.containers import Grid, Horizontal, Vertical
from textual.binding import Binding
from textual import on
from rich.text import Text
from posting.variables import get_variables

class EnvironmentDialog(ModalScreen[bool]):

    """Screen with a list of environment variables."""

    DEFAULT_CSS = """
    EnvironmentDialog {
        align: center middle;
        height: auto;
        & #environment-buttons {
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

    def on_mount(self) -> None:
        # Set up cells
        items = list(get_variables().items())
        table: DataTable = self.query_one("#table", DataTable)
        table.add_columns("Key", "Value")
        for row in items:
            # Adding styled and justified `Text` objects instead of plain strings.
            styled_row = [
                Text(str(cell), style="italic #03AC13", justify="right") for cell in row
            ]
            table.add_row(*styled_row)

        # Set up escape binding
        self._bindings.bind("escape", "screen.dismiss(False)")

    def compose(self) -> ComposeResult:
        with Vertical(id="environment-screen", classes="modal-body") as container:
            container.border_title = "Environment variables"
            yield DataTable(id="table")
            with Horizontal(id="environment-buttons"):
                yield Button("Ok \\[ESC]", id="ok-button")

    @on(Button.Pressed, "#ok-button")
    def confirm(self) -> None:
        self.dismiss(True)

    def action_move_focus(self) -> None:
        # It's enough to just call focus_next here as there are only two buttons.
        self.screen.focus_next()
