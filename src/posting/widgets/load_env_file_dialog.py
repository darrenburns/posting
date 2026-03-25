"""A modal screen for loading environment variables from a file."""

from __future__ import annotations

import os
from pathlib import Path
from typing import TYPE_CHECKING, Any

from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.screen import ModalScreen
from textual.widgets import Button, Input, Static
from textual_autocomplete import DropdownItem, PathAutoComplete, TargetState

from posting.locations import config_directory
from posting.variables import load_variables, update_variables
from posting.widgets.center_middle import CenterMiddle
from posting.widgets.input import PostingInput

if TYPE_CHECKING:
    from posting.app import Posting as PostingApp


def is_env_file_candidate(path: Path) -> bool:
    """Return True if the path looks like an environment file."""
    name = path.name
    return name == ".env" or name.endswith(".env") or name.startswith(".env.")


def resolve_env_file_path(env_file_path: str | Path, working_directory: Path) -> Path:
    """Resolve user input to an absolute path for loading."""
    env_path = Path(env_file_path).expanduser()
    if not env_path.is_absolute():
        env_path = working_directory / env_path
    return env_path.resolve(strict=False)


def load_env_file(
    app: "PostingApp" | Any,
    env_file_path: str | Path,
    *,
    working_directory: Path | None = None,
) -> bool:
    """Validate and load an environment file into the app session."""
    resolved_path = resolve_env_file_path(
        env_file_path,
        working_directory=working_directory or Path.cwd(),
    )

    if not resolved_path.exists():
        app.notify(f"Environment file not found: {resolved_path}", severity="error")
        return False

    if not resolved_path.is_file():
        app.notify(f"Environment path is not a file: {resolved_path}", severity="error")
        return False

    app.environment_files = (resolved_path,)
    load_variables(
        app.environment_files,
        app.settings.use_host_environment,
        avoid_cache=True,
    )
    update_variables(app.session_env)
    app.env_changed_signal.publish(None)
    app.notify(f"Loaded environment from: {resolved_path}")
    return True


class EnvFilePathAutoComplete(PathAutoComplete):
    """Autocomplete for filesystem paths that emphasizes environment files."""

    def __init__(
        self,
        target: Input | str,
        working_directory: Path,
        *,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        self.working_directory = working_directory
        self._config_directory = config_directory()
        super().__init__(
            target=target,
            path=working_directory,
            show_dotfiles=True,
            name=name,
            id=id,
            classes=classes,
            disabled=disabled,
        )

    def _directory_entries(self, directory: Path) -> list[os.DirEntry[str]]:
        cache_key = str(directory)
        cached_entries = self._directory_cache.get(cache_key)
        if cached_entries is not None:
            return cached_entries

        try:
            entries = list(os.scandir(directory))
        except OSError:
            return []

        self._directory_cache[cache_key] = entries
        return entries

    def _directory_item(self, value: str) -> DropdownItem:
        return DropdownItem(value, prefix=self.folder_prefix)

    def _file_item(self, value: str) -> DropdownItem:
        return DropdownItem(value, prefix=self.file_prefix)

    def _get_input_before_cursor(self, target_state: TargetState) -> str:
        return target_state.text[: target_state.cursor_position]

    def _resolve_browse_directory(self, current_input: str) -> Path:
        if "/" not in current_input:
            return self.working_directory

        parent_fragment = current_input[: current_input.rindex("/") + 1]
        if parent_fragment.startswith("~/"):
            return Path(parent_fragment).expanduser()
        if parent_fragment.startswith("/"):
            return Path(parent_fragment)
        return (self.working_directory / parent_fragment).resolve(strict=False)

    def _build_directory_candidates(self, directory: Path) -> list[DropdownItem]:
        results: list[DropdownItem] = []
        for entry in self._directory_entries(directory):
            entry_path = Path(entry.path)
            if entry.is_dir():
                results.append(self._directory_item(f"{entry.name}/"))
            elif is_env_file_candidate(entry_path):
                results.append(self._file_item(entry.name))
        results.sort(key=self._candidate_sort_key)
        return results

    def _build_empty_candidates(self) -> list[DropdownItem]:
        cwd_candidates = self._build_directory_candidates(self.working_directory)

        seen_paths = {
            (self.working_directory / candidate.value).resolve(strict=False)
            for candidate in cwd_candidates
            if not candidate.value.endswith("/")
        }
        config_candidates: list[DropdownItem] = []
        for entry in self._directory_entries(self._config_directory):
            entry_path = Path(entry.path)
            if entry.is_dir() or not is_env_file_candidate(entry_path):
                continue

            value = str(entry_path.resolve())
            resolved_path = Path(value)
            if resolved_path in seen_paths:
                continue
            seen_paths.add(resolved_path)
            config_candidates.append(self._file_item(value))

        config_candidates.sort(key=self._candidate_sort_key)
        return [*cwd_candidates, *config_candidates]

    def _candidate_sort_key(self, item: DropdownItem) -> tuple[bool, bool, str]:
        value = item.value
        name = value.rstrip("/").split("/")[-1]
        is_directory = value.endswith("/")
        is_dotfile = name.startswith(".")
        return (not is_directory, not is_dotfile, name.lower())

    def get_candidates(self, target_state: TargetState) -> list[DropdownItem]:
        current_input = self._get_input_before_cursor(target_state)
        if not current_input:
            return self._build_empty_candidates()

        browse_directory = self._resolve_browse_directory(current_input)
        return self._build_directory_candidates(browse_directory)

    def get_search_string(self, target_state: TargetState) -> str:
        current_input = self._get_input_before_cursor(target_state)
        if "/" in current_input:
            _before, _sep, after = current_input.rpartition("/")
            return after
        return current_input

    def should_show_dropdown(self, search_string: str) -> bool:
        return super().should_show_dropdown(search_string) or (
            search_string == ""
            and self.target.value != ""
            and self.option_list.option_count > 0
        )


def show_load_env_file_dialog(app: "PostingApp") -> None:
    """Show the load environment file dialog.

    This is the entry point for the command palette command.

    Args:
        app: The Posting app instance.
    """

    def on_submit(env_file_path: str | None) -> None:
        if env_file_path is None:
            return

        load_env_file(app, env_file_path)

    app.push_screen(LoadEnvFileDialog(working_directory=Path.cwd()), on_submit)


class LoadEnvFileDialog(ModalScreen[str | None]):
    """Screen for loading environment variables from a file."""

    DEFAULT_CSS = """
    LoadEnvFileDialog {
        align: center middle;
        EnvFilePathAutoComplete {
            layer: above;
        }

        & #env-input {
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
        self.working_directory = working_directory or Path.cwd()
        self.path_auto_complete: EnvFilePathAutoComplete | None = None
        super().__init__(name=name, id=id, classes=classes)

    def on_mount(self) -> None:
        self._bindings.bind("escape", "screen.dismiss(None)")

        input_widget = self.query_one("#env-input", Input)
        self.path_auto_complete = EnvFilePathAutoComplete(
            target=input_widget,
            working_directory=self.working_directory,
        )
        self.screen.mount(self.path_auto_complete)
        input_widget.focus()

    def compose(self) -> ComposeResult:
        with Vertical(classes="modal-body") as container:
            container.border_title = "Load Environment File"
            yield Static("Enter a path to an environment file:")
            yield PostingInput(
                placeholder=".env, path/to/file.env, ~/posting.env",
                id="env-input",
            )

            with Horizontal(id="env-buttons"):
                yield Button("Load \\[Enter]", id="load-button", variant="primary")
                yield Button("Cancel \\[ESC]", id="cancel-button")

    @on(Button.Pressed, "#load-button")
    @on(Input.Submitted, "#env-input")
    def confirm(self) -> None:
        """Handle load button press or enter key in input."""
        input_widget = self.query_one("#env-input", Input)
        env_path = input_widget.value.strip()
        if not env_path:
            self.dismiss(None)
            return

        resolved_path = resolve_env_file_path(
            env_path, working_directory=self.working_directory
        )
        if not resolved_path.exists():
            self.app.notify(
                f"Environment file not found: {resolved_path}",
                severity="error",
            )
            return

        if not resolved_path.is_file():
            self.app.notify(
                f"Environment path is not a file: {resolved_path}",
                severity="error",
            )
            return

        self.dismiss(str(resolved_path))

    @on(Button.Pressed, "#cancel-button")
    def cancel(self) -> None:
        """Handle cancel button press."""
        self.dismiss(None)

    def action_move_focus(self) -> None:
        self.screen.focus_next()
