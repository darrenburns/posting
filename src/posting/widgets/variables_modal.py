from __future__ import annotations

from dataclasses import dataclass

from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.screen import ModalScreen
from textual.widgets import DataTable, Footer, Input, Label

from posting.variables import get_variables, load_variables, update_variables
from posting.widgets.variable_edit_modal import VariableEdit, VariableEditModal

# Substrings that mark a variable as sensitive. The screen masks matching values
# until you press 's', so opening it during a screen share or a recording does not
# put a token or a password on the wall.
SENSITIVE_MARKERS = ("secret", "password", "token", "passwd", "api_key", "apikey")

# Cap the undo history so a long session cannot grow it without bound.
MAX_HISTORY = 50


@dataclass
class VariableChange:
    """The state of one variable before a change, so it can be restored.

    Edits, additions and reverts all answer the same question: did this name
    have a session override before, and what was it?
    """

    name: str
    had_override: bool
    previous_value: str | None


def is_sensitive(name: str) -> bool:
    lowered = name.lower()
    return any(marker in lowered for marker in SENSITIVE_MARKERS)


def build_rows(
    variables: dict[str, object],
    session_names: set[str],
    show_sensitive: bool = False,
    filter_query: str = "",
) -> list[tuple[str, str, str]]:
    """Turn the variable store into the (name, value, source) rows to display.

    Takes no widget, so the masking and filtering rules can be tested without
    starting an app.
    """
    query = filter_query.lower()
    rows: list[tuple[str, str, str]] = []
    for name, value in sorted(variables.items()):
        if query and query not in name.lower():
            continue
        rendered = str(value)
        if is_sensitive(name) and not show_sensitive:
            rendered = "•" * min(len(rendered), 12)
        source = "session" if name in session_names else "env file"
        rows.append((name, rendered, source))
    return rows


class VariablesModal(ModalScreen[None]):
    """Lists every variable a request can resolve, and lets you change them.

    The values come from `get_variables()`, the same store the request
    templating reads, so a row shows what `$FOO` will expand to. A session
    variable (set by a script through `posting.set_variable`, or here) hides an
    environment file variable of the same name, so the table labels which one
    you are looking at. Without that label a value that looks stale gives you
    nothing to go on.
    """

    DEFAULT_CSS = """
    VariablesModal {
        align: center middle;

        & .modal-body {
            width: 80%;
            max-width: 100;
            height: 80%;
            max-height: 30;
            padding: 0 1;
        }

        & Input {
            border: none;
            padding: 0 1;
            height: 1;
            background: transparent;

            &:focus {
                border: none;
            }
        }

        & DataTable {
            height: 1fr;
            background: transparent;
        }

        & Footer {
            background: transparent;
        }

        & .empty-message {
            padding: 1 2;
            color: $text-muted;
        }
    }
    """

    BINDINGS = [
        Binding("escape", "close_or_defocus", "Close", show=False),
        Binding("slash", "focus_filter", "Filter"),
        Binding("e", "edit_value", "Edit"),
        Binding("a", "add_variable", "Add"),
        Binding("d", "revert_override", "Revert"),
        Binding("s", "toggle_sensitive", "Secrets"),
        Binding("c", "copy_value", "Copy"),
        Binding("$", "copy_reference", "Copy $NAME"),
        Binding("u", "undo", "Undo"),
        Binding("ctrl+r", "redo", "Redo"),
        Binding("j", "cursor_down", "Down", show=False),
        Binding("k", "cursor_up", "Up", show=False),
    ]

    def __init__(self) -> None:
        super().__init__()
        self.show_sensitive = False
        self.filter_query = ""

    def compose(self) -> ComposeResult:
        with Vertical(classes="modal-body") as container:
            container.border_title = "Variables"
            if get_variables():
                yield Input(placeholder="Filter variables…", id="variable-filter")
                table: DataTable[str] = DataTable(
                    zebra_stripes=True, cursor_type="row", id="variables-table"
                )
                table.add_columns("Variable", "Value", "Source")
                yield table
            else:
                # Still yield the footer below, so the keys remain discoverable
                # even when there is nothing to list yet.
                yield Label(
                    "No variables loaded.\n\n"
                    "Load an environment file with the "
                    "'environment: Load env file' command, or pass --env on startup.\n"
                    "You can also add one here with 'a'.",
                    classes="empty-message",
                )
        yield Footer(show_command_palette=False)

    def on_mount(self) -> None:
        if self.query("#variables-table"):
            self.populate()
            self.query_one(DataTable).focus()

    @property
    def session_names(self) -> set[str]:
        """Names that were set from a script, rather than an environment file."""
        return set(getattr(self.app, "session_env", {}))

    def populate(self) -> None:
        table: DataTable[str] = self.query_one("#variables-table", DataTable)
        table.clear()
        rows = build_rows(
            get_variables(),
            self.session_names,
            show_sensitive=self.show_sensitive,
            filter_query=self.filter_query,
        )
        for name, rendered, source in rows:
            table.add_row(name, rendered, source, key=name)

    @on(Input.Changed, "#variable-filter")
    def on_filter_changed(self, event: Input.Changed) -> None:
        self.filter_query = event.value
        self.populate()

    def action_focus_filter(self) -> None:
        """`/` jumps to the filter, matching the collection browser's search key.

        Single-letter bindings are safe alongside this: an `Input` consumes
        printable keys, so typing "secret" does not trip the `s` binding.
        """
        filter_input = self.query("#variable-filter")
        if filter_input:
            filter_input.first(Input).focus()

    def action_close_or_defocus(self) -> None:
        """Escape backs out of the filter first, and only then closes.

        Otherwise escaping a mistyped filter throws away the whole screen.
        """
        if isinstance(self.focused, Input):
            self.query_one(DataTable).focus()
        else:
            self.dismiss(None)

    def action_toggle_sensitive(self) -> None:
        self.show_sensitive = not self.show_sensitive
        self.populate()
        self.notify(
            "Secrets shown" if self.show_sensitive else "Secrets hidden",
            timeout=2,
        )

    @property
    def selected_name(self) -> str | None:
        """The variable name under the cursor, if the table has a selection."""
        if not self.query("#variables-table"):
            return None
        table: DataTable[str] = self.query_one("#variables-table", DataTable)
        if table.cursor_row < 0 or table.row_count == 0:
            return None
        return table.coordinate_to_cell_key((table.cursor_row, 0)).row_key.value

    def action_copy_value(self) -> None:
        """Copy the real value, never the masked placeholder."""
        name = self.selected_name
        if name is None:
            return
        self.app.copy_to_clipboard(str(get_variables().get(name, "")))
        self.notify(f"Copied value of {name}", timeout=2)

    def action_copy_reference(self) -> None:
        """Copy `$NAME` - what you paste into a URL, header or body."""
        name = self.selected_name
        if name is None:
            return
        self.app.copy_to_clipboard(f"${name}")
        self.notify(f"Copied ${name}", timeout=2)

    # ------------------------------------------------------------------ undo

    def snapshot(self, name: str) -> VariableChange:
        """Capture the current session state of `name`, before changing it."""
        had = name in self.app.session_env
        return VariableChange(
            name=name,
            had_override=had,
            previous_value=str(self.app.session_env[name]) if had else None,
        )

    def restore(self, change: VariableChange) -> None:
        """Put a variable back into the state described by `change`."""
        if change.had_override:
            self.app.session_env[change.name] = change.previous_value
            update_variables(self.app.session_env)
        else:
            # No override to put back, so reload the environment file value.
            # The flat store still holds the value that was overriding it.
            self.app.session_env.pop(change.name, None)
            load_variables(
                self.app.environment_files,
                self.app.settings.use_host_environment,
                avoid_cache=True,
            )
            update_variables(self.app.session_env)
        self.app.env_changed_signal.publish(None)
        self.populate()

    def record(self, change: VariableChange) -> None:
        """Push an undo entry. A fresh change invalidates any pending redo."""
        history = self.app.variable_history
        history.append(change)
        del history[:-MAX_HISTORY]
        self.app.variable_redo_stack.clear()

    def action_undo(self) -> None:
        history = self.app.variable_history
        if not history:
            self.notify("Nothing to undo", severity="warning", timeout=2)
            return
        change = history.pop()
        # Remember the current state so the undo itself can be redone.
        self.app.variable_redo_stack.append(self.snapshot(change.name))
        self.restore(change)
        self.notify(f"Undid change to {change.name}", timeout=2)

    def action_redo(self) -> None:
        redo_stack = self.app.variable_redo_stack
        if not redo_stack:
            self.notify("Nothing to redo", severity="warning", timeout=2)
            return
        change = redo_stack.pop()
        # Record the state we are leaving, not the one we are moving to - a
        # history entry describes what undo should return to. Pushed straight on
        # rather than via record(), which would wipe the rest of the redo stack
        # and make repeated redo impossible.
        self.app.variable_history.append(self.snapshot(change.name))
        self.restore(change)
        self.notify(f"Redid change to {change.name}", timeout=2)

    # --------------------------------------------------------------- mutation

    def set_session_variable(self, name: str, value: str) -> None:
        """Store a session override and refresh everything that reads variables."""
        self.record(self.snapshot(name))
        self.app.session_env[name] = value
        update_variables(self.app.session_env)
        self.app.env_changed_signal.publish(None)
        self.populate()

    def action_edit_value(self) -> None:
        name = self.selected_name
        if name is None:
            return
        current = str(get_variables().get(name, ""))

        def on_close(result: VariableEdit | None) -> None:
            if result is not None:
                self.set_session_variable(result.name, result.value)
                self.notify(f"{result.name} set for this session", timeout=3)

        self.app.push_screen(VariableEditModal(name=name, value=current), on_close)

    def action_add_variable(self) -> None:
        def on_close(result: VariableEdit | None) -> None:
            if result is None:
                return
            self.set_session_variable(result.name, result.value)
            self.notify(f"{result.name} added for this session", timeout=3)

        self.app.push_screen(VariableEditModal(is_new=True), on_close)

    def action_revert_override(self) -> None:
        """Drop a session override so the environment file value applies again.

        The variable store is flat, so dropping the key from `session_env` leaves
        the overriding value behind. Reload the files and re-overlay the rest of
        the session, the same way the file watcher does.
        """
        name = self.selected_name
        if name is None:
            return
        if name not in self.app.session_env:
            # An environment file value cannot go away from here: the file is
            # the source of truth, so the next reload would bring it back.
            # Name the two things that do work instead.
            self.notify(
                f"{name} comes from an environment file, so there is nothing to "
                f"revert. Override it with 'e', or edit the file itself.",
                severity="warning",
                timeout=5,
            )
            return

        self.record(self.snapshot(name))
        del self.app.session_env[name]
        load_variables(
            self.app.environment_files,
            self.app.settings.use_host_environment,
            avoid_cache=True,
        )
        update_variables(self.app.session_env)
        self.app.env_changed_signal.publish(None)
        self.populate()

        restored = name in get_variables()
        self.notify(
            f"{name} restored from environment file"
            if restored
            else f"{name} removed",
            timeout=3,
        )

    def action_cursor_down(self) -> None:
        self.query_one(DataTable).action_cursor_down()

    def action_cursor_up(self) -> None:
        self.query_one(DataTable).action_cursor_up()
