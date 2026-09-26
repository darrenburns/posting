"""File uploads and variable completion share the form value input."""

from pathlib import Path
from textual_autocomplete import DropdownItem, TargetState
from posting.widgets.variable_autocomplete import VariableAutoComplete
from posting.widgets.variable_input import VariableInput


class FormValueAutoComplete(VariableAutoComplete):
    @staticmethod
    def is_upload(state: TargetState) -> bool:
        return (
            state.text.startswith("@")
            and not state.text.startswith("@@")
            and state.cursor_position > 0
        )

    def get_candidates(self, state: TargetState) -> list[DropdownItem]:
        if not self.is_upload(state):
            return super().get_candidates(state)
        prefix = state.text[1 : state.cursor_position]
        directory_text, slash, _ = prefix.rpartition("/")
        directory = Path(directory_text + slash).expanduser() if slash else Path(".")
        if not directory.is_absolute():
            directory = self.app.collection.path / directory
        try:
            entries = sorted(
                directory.iterdir(), key=lambda p: (not p.is_dir(), p.name.lower())
            )
            return [
                DropdownItem(
                    "@"
                    + directory_text
                    + slash
                    + entry.name
                    + ("/" if entry.is_dir() else ""),
                    prefix="📂 " if entry.is_dir() else "📄 ",
                )
                for entry in entries
            ]
        except OSError:
            return []

    def get_search_string(self, state: TargetState) -> str:
        return (
            state.text[: state.cursor_position]
            if self.is_upload(state)
            else super().get_search_string(state)
        )

    def apply_completion(self, value: str, state: TargetState) -> None:
        if self.is_upload(state):
            self.target.value = value
            self.target.cursor_position = len(value)
            self.post_completion()
        else:
            super().apply_completion(value, state)

    def post_completion(self) -> None:
        super().post_completion()
        if self.target.value.startswith("@") and self.target.value.endswith("/"):
            self.call_after_refresh(self._handle_target_update)


class FormValueInput(VariableInput):
    autocomplete_class = FormValueAutoComplete
