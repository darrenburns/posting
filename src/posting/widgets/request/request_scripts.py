from pathlib import Path
import shlex
import subprocess
from typing import Any
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import VerticalScroll
from textual.widget import Widget
from textual.widgets import Input, Label
from textual_autocomplete import AutoComplete, DropdownItem, TargetState

from posting.collection import Scripts
from posting.config import SETTINGS
from posting.scripts import uncache_module


class ScriptPathInput(Input):
    BINDINGS = [
        Binding(
            "ctrl+e",
            "open_in_editor",
            "To editor",
            tooltip="Open this script in the configured $EDITOR.",
        ),
        Binding(
            "ctrl+p",
            "open_in_pager",
            "To pager",
            tooltip="Open this script in the configured $PAGER.",
        ),
    ]

    def __init__(
        self,
        collection_root: Path,
        *args: Any,
        **kwargs: Any,
    ) -> None:
        super().__init__(*args, **kwargs)
        self.collection_root = collection_root

    def _get_script_path(self) -> Path | None:
        """
        Get the script path from the input value.

        Handles the `path/to/script.py:function_name` syntax and resolves
        relative paths against the collection root.

        Returns:
            The resolved script path if it exists, None otherwise.
        """
        # Handle the `path/to/script.py:function_name` syntax
        value = self.value.strip()
        if ":" in value:
            value, _function_name = value.split(":")

        # Paths are interpreted relative to the collection root
        script_path = Path(value)
        if not script_path.is_absolute():
            script_path = self.collection_root / script_path

        # Ensure the script exists, and notify the user if it doesn't
        if not script_path.exists():
            self.app.notify(
                severity="error",
                title="Invalid script path",
                message=f"The script file '{script_path}' does not exist.",
            )
            return None

        return script_path

    def _open_with_command(self, command_name: str, command_setting: str) -> None:
        """
        Open the script in the specified command.

        Args:
            command_name: The name of the command to use (for display purposes).
            command_setting: The name of the setting to retrieve the command from.
        """
        command = SETTINGS.get().__getattribute__(command_setting)
        if not command:
            self.app.notify(
                severity="warning",
                title=f"No {command_name} configured",
                message=f"Set the [b]${command_setting.upper()}[/b] environment variable.",
            )
            return

        script_path = self._get_script_path()
        if not script_path:
            return

        command_args = shlex.split(command)
        command_args.append(str(script_path))

        with self.app.suspend():
            try:
                subprocess.call(command_args)
            except OSError:
                command_string = shlex.join(command_args)
                self.app.notify(
                    severity="error",
                    title=f"Can't run {command_name} command",
                    message=f"The command [b]{command_string}[/b] failed to run.",
                )

        # We're back in Posting, uncache the edited module as it may have
        # been updated, and notify the user that we're aware of the change.
        uncache_module(str(script_path))

    def action_open_in_editor(self) -> None:
        """
        Open the script in the configured editor.

        This action is triggered when the user presses Ctrl+E.
        """
        self._open_with_command("editor", "editor")

    def action_open_in_pager(self) -> None:
        """
        Open the script in the configured pager.

        This action is triggered when the user presses Ctrl+P.
        """
        self._open_with_command("pager", "pager")


class RequestScripts(VerticalScroll):
    """Collections can contain a scripts folder.

    This widget is about linking scripts to requests.

    A script is a Python file which may contain functions
    named `pre_request` and/or `post_response`.

    Neither function is required, but if present and the path
    is supplied, Posting will automatically fetch the function
    from the file and execute it at the appropriate time.

    You can also specify the name of the function as a suffix
    after the path, separated by a colon.

    Example:
    ```
    scripts/on_request.py:prepare_auth
    scripts/on_response.py:log_response
    ```

    The API for scripts is under development and will likely change.

    The goal is to allow developers to attach scripts to requests,
    which will then be executed when the request is made, and when the
    response is received. This includes performing assertions, using
    plain assert statements, and hooks for deeper integration with Posting.
    For example, sending a notification when a request fails, or logging
    the response to a file.
    """

    DEFAULT_CSS = """
    RequestScripts {
        padding: 0 2;
        & Input {
            margin-bottom: 1;
        }
    }
    """

    def __init__(
        self,
        *children: Widget,
        collection_root: Path,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(
            *children, name=name, id=id, classes=classes, disabled=disabled
        )
        self.collection_root = collection_root

    def compose(self) -> ComposeResult:
        self.can_focus = False

        yield Label("Setup script [dim]optional[/dim]")
        yield ScriptPathInput(
            collection_root=self.collection_root,
            placeholder="Collection-relative path to setup script",
            id="setup-script",
        )

        yield Label("Pre-request script [dim]optional[/dim]")
        yield ScriptPathInput(
            collection_root=self.collection_root,
            placeholder="Collection-relative path to pre-request script",
            id="pre-request-script",
        )

        yield Label("Post-response script [dim]optional[/dim]")
        yield ScriptPathInput(
            collection_root=self.collection_root,
            placeholder="Collection-relative path to post-response script",
            id="post-response-script",
        )

    def on_mount(self) -> None:
        auto_complete_setup = AutoComplete(
            candidates=self.get_script_candidates,
            target=self.query_one("#setup-script", Input),
        )
        auto_complete_pre_request = AutoComplete(
            candidates=self.get_script_candidates,
            target=self.query_one("#pre-request-script", Input),
        )
        auto_complete_post_response = AutoComplete(
            candidates=self.get_script_candidates,
            target=self.query_one("#post-response-script", Input),
        )

        self.screen.mount(auto_complete_setup)
        self.screen.mount(auto_complete_pre_request)
        self.screen.mount(auto_complete_post_response)

    def get_script_candidates(self, state: TargetState) -> list[DropdownItem]:
        scripts: list[DropdownItem] = []
        for script in self.collection_root.glob("**/*.py"):
            scripts.append(DropdownItem(str(script.relative_to(self.collection_root))))
        return scripts

    def load_scripts(self, scripts: Scripts) -> None:
        self.query_one("#setup-script", Input).value = scripts.setup or ""
        self.query_one("#pre-request-script", Input).value = scripts.on_request or ""
        self.query_one("#post-response-script", Input).value = scripts.on_response or ""

    def to_model(self) -> Scripts:
        return Scripts(
            setup=self.query_one("#setup-script", Input).value or None,
            on_request=self.query_one("#pre-request-script", Input).value or None,
            on_response=self.query_one("#post-response-script", Input).value or None,
        )
