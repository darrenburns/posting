"""A one-time announcement of Posting 3, shown the first time the TUI starts."""

from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.screen import ModalScreen
from textual.widgets import Button, Static

from posting.locations import data_directory

MARKER_NAME = "posting-3-notice-shown"


def claim_posting3_notice() -> bool:
    """Record that the notice has been shown, returning True if it hadn't been yet.

    The marker is written before the notice appears, so a crash or kill while
    it is open still counts. If the marker can't be written, the notice is
    shown anyway rather than interfering with startup.
    """
    try:
        (data_directory() / MARKER_NAME).touch(exist_ok=False)
    except FileExistsError:
        return False
    except OSError:
        pass
    return True


class Posting3Notice(ModalScreen[None]):
    BINDINGS = [
        Binding("escape,enter", "dismiss", "Close", show=False),
    ]

    def compose(self) -> ComposeResult:
        with Vertical(classes="modal-body") as container:
            container.border_title = "Posting 3 beta"
            yield Static(
                "Posting 3 is available in beta. It's a rewrite in Go, shipped as a single binary.\n\n"
                "It reads the same collections, configuration and .env files, "
                "so you can try it alongside Posting 2.\n\n"
                "Install with Homebrew:\n"
                "  [b]brew install darrenburns/homebrew/posting@beta[/]\n"
                "Or with Go:\n"
                "  [b]go install github.com/darrenburns/posting/v3/cmd/posting@latest[/]\n\n"
                "Learn more at https://posting.sh/guide/migrating/\n\n"
                "[dim]This message won't be shown again.[/]"
            )
            yield Button("Close \\[esc]")

    def on_mount(self) -> None:
        self.query_one(Button).focus()

    @on(Button.Pressed)
    def close(self) -> None:
        self.dismiss()
