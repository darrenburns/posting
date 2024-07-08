from typing import Callable
from textual.widgets import Input, TextArea
from textual.widgets.text_area import Selection
from textual_autocomplete import (
    AutoComplete,
    DropdownItem,
    TargetState,
    MatcherFactoryType,
)


class VariableAutoComplete(AutoComplete):
    def __init__(
        self,
        target: Input | TextArea | str,
        candidates: list[DropdownItem] | Callable[[TargetState], list[DropdownItem]],
        variable_candidates: list[DropdownItem]
        | Callable[[TargetState], list[DropdownItem]],
        matcher_factory: MatcherFactoryType | None = None,
        prevent_default_enter: bool = True,
        prevent_default_tab: bool = True,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(
            target,
            candidates,
            matcher_factory,
            self._completion_strategy,
            self._search_string,
            prevent_default_enter,
            prevent_default_tab,
            name,
            id,
            classes,
            disabled,
        )
        self.variable_candidates = variable_candidates

    def is_cursor_within_variable(self, target_state: TargetState) -> bool:
        # Find the last '$' before the cursor
        cursor = target_state.selection.end[1]
        last_dollar = target_state.text.rfind("$", 0, cursor)

        if last_dollar == -1:
            return False

        # Check if there's any whitespace between the last '$' and the cursor
        return " " not in target_state.text[last_dollar:cursor]

    def find_variable_start(self, target_state: TargetState) -> int:
        return target_state.text.rfind("$", 0, target_state.selection.end[1])

    def find_variable_end(self, target_state: TargetState) -> int:
        for i in range(target_state.selection.end[1], len(target_state.text)):
            if target_state.text[i].isspace():
                return i
        return len(target_state.text)

    def get_variable_at_cursor(self, target_state: TargetState) -> str | None:
        if not self.is_cursor_within_variable(target_state):
            return None

        start = self.find_variable_start(target_state)
        end = self.find_variable_end(target_state)

        return target_state.text[start:end]

    def get_candidates(self, target_state: TargetState) -> list[DropdownItem]:
        if self.is_cursor_within_variable(target_state):
            return self.get_variable_candidates(target_state)
        else:
            return super().get_candidates(target_state)

    def _completion_strategy(
        self, value: str, target_state: TargetState
    ) -> TargetState:
        if self.is_cursor_within_variable(target_state):
            # Replace the text from the variable start
            # with the completion text.
            start = self.find_variable_start(target_state)
            end = self.find_variable_end(target_state)
            old_value = target_state.text
            new_value = old_value[:start] + value + old_value[end:]

            # Move the cursor to the end of the inserted text
            new_column = start + len(value)
            return TargetState(
                text=new_value,
                selection=Selection.cursor((0, new_column)),
            )
        else:
            # Replace the entire contents
            return TargetState(
                text=value,
                selection=Selection.cursor((0, len(value))),
            )

    def _search_string(self, target_state: TargetState) -> str:
        if self.is_cursor_within_variable(target_state):
            return self.get_variable_at_cursor(target_state) or ""
        else:
            return target_state.text

    def get_variable_candidates(self, target_state: TargetState) -> list[DropdownItem]:
        candidates = self.variable_candidates
        return candidates(target_state) if callable(candidates) else candidates
