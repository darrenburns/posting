"""A modal screen for loading environment variables from a file."""

from pathlib import Path
from typing import TYPE_CHECKING
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.screen import ModalScreen
from textual.widgets import Button, Input, Static
from textual import on
from posting.locations import config_directory
from posting.variables import load_variables, update_variables
from textual.widgets import Select
from textual.widgets import Select as PostingSelect

if TYPE_CHECKING:
    from posting.app import Posting as PostingApp


def show_load_env_file_dialog(app: "PostingApp") -> None:
    """Show the load environment file dialog.

    This is the entry point for the command palette command.

    Args:
        app: The Posting app instance.
    """
    def on_submit(env_file_path: str | None) -> None:
        if env_file_path is None:
            return

        env_path = Path(env_file_path).expanduser()
        if not env_path.exists():
            app.notify(f"Environment file not found: {env_path}", severity="error")
            return

        app.environment_files = (env_path,)
        load_variables(app.environment_files, app.settings.use_host_environment, avoid_cache=True)
        update_variables(app.session_env)
        app.env_changed_signal.publish(None)
        app.notify(f"Loaded environment from: {env_path}")

    app.push_screen(
        LoadEnvFileDialog(working_directory=Path.cwd()),
        on_submit
    )


class LoadEnvFileDialog(ModalScreen[str | None]):
    """Screen for loading environment variables from a file."""

    DEFAULT_CSS = """
    LoadEnvFileDialog {
        align: center middle;
        height: auto;
        & #env-input {
            margin-bottom: 1;
        }
        & #env-select {
            margin-bottom: 1;
        }
        & #env-buttons {
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
        working_directory: Path | None = None,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
    ) -> None:
        # Scan BEFORE super().__init__ so compose() can access the results
        self.working_directory = working_directory or Path.cwd()
        self._existing_env_files: list[tuple[str, Path]] = []
        self._scan_env_files()

        super().__init__(name=name, id=id, classes=classes)

    def on_mount(self) -> None:
        # Bind escape key
        self.app.bind("escape", "screen.dismiss(None)")

    def _scan_env_files(self) -> None:
        """Scan for existing .env files in the working and config directories."""
        config_dir = config_directory()
        working_dir = self.working_directory

        # Scan working directory - add [cwd] prefix (escaped for Textual)
        if working_dir.exists() and working_dir.is_dir():
            for f in working_dir.iterdir():
                if f.is_file() and f.name.endswith(".env"):
                    display_name = f"\\[ working dir ] {f.name}"
                    self._existing_env_files.append((display_name, f.absolute()))

        # Scan config directory - add [config] prefix (escaped for Textual)
        if config_dir.exists() and config_dir.is_dir():
            for f in config_dir.iterdir():
                if f.is_file() and f.name.endswith(".env"):
                    display_name = f"\\[ config dir  ] {f.name}"
                    self._existing_env_files.append((display_name, f.absolute()))

        # Sort by display name
        self._existing_env_files.sort(key=lambda x: x[0])

    def compose(self) -> ComposeResult:
        with Vertical(id="env-file-screen", classes="modal-body") as container:
            container.border_title = "Load Environment File"

            # Show dropdown with existing env files
            if self._existing_env_files:
                yield Static("Select .env file or enter path below:")
                # Use display name for showing, path as value
                options = [(name, str(path)) for name, path in self._existing_env_files]
                yield PostingSelect(
                    options,
                    id="env-select",
                )
            else:
                yield Static("No .env files found in current directory.")
                yield PostingSelect(
                    [],
                    id="env-select",
                )

            yield Static("Or enter path to environment file:")
            yield Input(placeholder=".env, path/to/file.env", id="env-input")

            with Horizontal(id="env-buttons"):
                yield Button("Load \\[Enter]", id="load-button", variant="primary")
                yield Button("Cancel \\[ESC]", id="cancel-button")

    @on(PostingSelect.Changed, "#env-select")
    def on_select_changed(self, event: PostingSelect.Changed) -> None:
        """When dropdown selection changes, populate the input field."""
        if isinstance(event.value, str):
            # value is now the path directly (from options format: (name, path))
            input_widget = self.query_one("#env-input", Input)
            input_widget.value = event.value

    @on(Button.Pressed, "#load-button")
    @on(Input.Submitted, "#env-input")
    def confirm(self) -> None:
        """Handle load button press or enter key in input."""
        input_widget = self.query_one("#env-input", Input)
        env_path = input_widget.value.strip()
        if env_path:
            self.dismiss(env_path)
        else:
            self.dismiss(None)

    @on(Button.Pressed, "#cancel-button")
    def cancel(self) -> None:
        """Handle cancel button press."""
        self.dismiss(None)

    def action_move_focus(self) -> None:
        # Cycle focus between input and buttons
        self.screen.focus_next()
