from __future__ import annotations

import re

from textual import on
from textual.binding import Binding
from textual.widgets.data_table import RowKey

from posting.variables import get_variables
from posting.widgets.datatable import PostingDataTable
from posting.widgets.input import PostingInput
from posting.widgets.key_value import KeyValueEditor, KeyValueInput


SENSITIVE_MARKERS = ("secret", "password", "token", "passwd", "api_key", "apikey")


def is_sensitive(name: str) -> bool:
    return any(marker in name.lower() for marker in SENSITIVE_MARKERS)


def build_rows(
    variables: dict[str, object],
    session_names: set[str],
    show_sensitive: bool = False,
    filter_query: str = "",
) -> list[tuple[str, str, str]]:
    rows = []
    for name, value in sorted(variables.items()):
        if filter_query.lower() not in name.lower():
            continue
        rendered = "•" * 12 if is_sensitive(name) and not show_sensitive else str(value)
        source = "session" if name in session_names else "env file"
        rows.append((name, rendered, source))
    return rows


class VariablesTable(PostingDataTable):
    BINDINGS = [
        Binding("c,y", "copy_cell", "Copy value"),
        Binding("$", "copy_reference", "Copy $NAME"),
    ]

    def on_mount(self) -> None:
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.add_columns("Variable", "Value", "Source")

    @property
    def selected_name(self) -> str | None:
        if not 0 <= self.cursor_row < self.row_count:
            return None
        return self.coordinate_to_cell_key((self.cursor_row, 0)).row_key.value

    def action_copy_cell(self) -> None:
        """Copy the underlying value, without exposing it in a notification."""
        name = self.selected_name
        if name is not None:
            self.app.copy_to_clipboard(str(get_variables().get(name, "")))
            self.notify(f"Copied value of {name}", timeout=2)

    def action_copy_reference(self) -> None:
        name = self.selected_name
        if name is not None:
            self.app.copy_to_clipboard(f"${name}")
            self.notify(f"Copied ${name}", timeout=2)


class VariablesEditor(KeyValueEditor):
    """Use the shared editing flow with unique names and session overrides."""

    DEFAULT_CSS = """
    VariablesEditor {
        height: 1fr;
        & VariablesTable {
            height: 1fr;
            background: transparent;
        }
    }
    """

    BINDINGS = [
        Binding("e", "edit_row('value')", "Edit", show=False),
        Binding("a", "add_variable", "Add"),
        Binding("d", "revert_override", "Revert"),
        Binding("s", "toggle_sensitive", "Secrets"),
        Binding("u", "undo", "Undo"),
        Binding("ctrl+r", "redo", "Redo"),
    ]

    def __init__(self) -> None:
        super().__init__(
            VariablesTable(id="variables-table"),
            KeyValueInput(
                PostingInput(placeholder="Name", id="variable-name"),
                PostingInput(placeholder="Value", id="variable-value"),
            ),
            empty_message=(
                "No variables to show. Add one below or load an environment file."
            ),
        )
        self.show_sensitive = False
        self.filter_query = ""

    def on_mount(self) -> None:
        self.app.env_changed_signal.subscribe(self, lambda _: self.populate())
        self.populate()

    def focus_editor(self) -> None:
        if self.table.row_count:
            self.table.focus()
        else:
            self.key_value_input.key_input.focus()

    def populate(self) -> None:
        # Keep a draft intact if a file reloads while the user is typing.
        # Exiting edit mode always refreshes from the latest variable store.
        if self._row_being_edited is not None:
            return
        table = self.query_one(VariablesTable)
        selected = table.selected_name
        table.clear()
        rows = build_rows(
            get_variables(),
            set(self.app.session_env),
            self.show_sensitive,
            self.filter_query,
        )
        for index, (name, value, source) in enumerate(rows):
            table.add_row(name, value, source, key=name, explicit_by_user=False)
            if name == selected:
                table.move_cursor(row=index)

    def get_value_to_edit(self, row_key: RowKey, displayed_value: str) -> str:
        return str(get_variables()[row_key.value])

    def enter_edit_mode(self, row_key: RowKey, focus_value: bool = False) -> None:
        self.key_value_input.value_input.password = (
            is_sensitive(row_key.value or "") and not self.show_sensitive
        )
        super().enter_edit_mode(row_key, focus_value=True)
        self.key_value_input.key_input.disabled = True

    def exit_edit_mode(self, revert: bool = False) -> None:
        super().exit_edit_mode(revert)
        self.key_value_input.key_input.disabled = False
        self.key_value_input.value_input.password = False
        # Row selection may immediately enter edit mode for another row.
        self.call_after_refresh(self.populate)

    @on(KeyValueInput.Change)
    def add_key_value_pair(self, event: KeyValueInput.Change) -> None:
        event.stop()
        event.prevent_default()
        name = event.key.strip()
        if not re.fullmatch(r"[a-zA-Z_]\w*", name):
            self.key_value_input.key_input.value = event.key
            self.key_value_input.value_input.value = event.value
            self.notify("Use a variable name such as MY_VARIABLE", severity="error")
            return
        if self._row_being_edited is not None:
            name = self._row_being_edited.value
            self.exit_edit_mode()
        self.app.variable_state.set(name, event.value)
        self.app.reload_variables()
        self.table.focus()
        self.notify(f"{name} set for this session", timeout=2)

    def action_add_variable(self) -> None:
        self.action_cancel_edit_row()
        self.key_value_input.key_input.focus()

    def action_toggle_sensitive(self) -> None:
        self.show_sensitive = not self.show_sensitive
        self.populate()
        if self._row_being_edited is not None:
            self.key_value_input.value_input.password = (
                is_sensitive(self._row_being_edited.value or "")
                and not self.show_sensitive
            )

    def action_revert_override(self) -> None:
        name = self.query_one(VariablesTable).selected_name
        if name is None:
            return
        self.action_cancel_edit_row()
        if not self.app.variable_state.revert(name):
            self.notify("No session override to revert", severity="warning", timeout=2)
            return
        self.app.reload_variables()

    def action_undo(self) -> None:
        self.action_cancel_edit_row()
        name = self.app.variable_state.undo()
        if name is None:
            self.notify("Nothing to undo", severity="warning", timeout=2)
        else:
            self.app.reload_variables()
            self.notify(f"Undid change to {name}", timeout=2)

    def action_redo(self) -> None:
        self.action_cancel_edit_row()
        name = self.app.variable_state.redo()
        if name is None:
            self.notify("Nothing to redo", severity="warning", timeout=2)
        else:
            self.app.reload_variables()
            self.notify(f"Redid change to {name}", timeout=2)
