from typing import Callable, Sequence
from textual.widgets import Input
from textual_autocomplete import (
    AutoComplete,
    DropdownItem,
    TargetState,
)

from posting.variables import (
    find_variable_end,
    find_variable_start,
    get_variable_at_cursor,
    get_variables,
    is_cursor_within_variable,
)


class VariableAutoComplete(AutoComplete):
    def __init__(
        self,
        target: Input | str,
        candidates: Sequence[DropdownItem | str]
        | Callable[[TargetState], list[DropdownItem]]
        | None = None,
        variable_candidates: Sequence[DropdownItem]
        | Callable[[TargetState], list[DropdownItem]]
        | None = None,
        prevent_default_enter: bool = True,
        prevent_default_tab: bool = True,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(
            target=target,
            candidates=candidates,
            prevent_default_enter=prevent_default_enter,
            prevent_default_tab=prevent_default_tab,
            name=name,
            id=id,
            classes=classes,
            disabled=disabled,
        )
        if variable_candidates is None:
            variable_candidates = [
                DropdownItem(main=f"${variable}") for variable in get_variables()
            ]
        self.variable_candidates = variable_candidates

    def get_candidates(self, target_state: TargetState) -> list[DropdownItem]:
        cursor = target_state.cursor_position
        text = target_state.text
        if is_cursor_within_variable(cursor, text):
            candidates = self.get_variable_candidates(target_state)
        else:
            candidates = super().get_candidates(target_state)
        return candidates

    def apply_completion(self, value: str, state: TargetState) -> None:
        """Modify the target state to reflect the completion.

        Only works in Inputs for now.
        """
        cursor = state.cursor_position
        text = state.text
        target: Input = self.target
        if is_cursor_within_variable(cursor, text):
            # Replace the text from the variable start
            # with the completion text.
            start = find_variable_start(cursor, text)
            end = find_variable_end(cursor, text)
            old_value = text
            new_value = old_value[:start] + value + old_value[end:]

            target.value = new_value
            target.cursor_position = start + len(value)
        else:
            target.value = value
            target.cursor_position = len(value)

        self.post_completion()

    def get_search_string(self, target_state: TargetState) -> str:
        cursor = target_state.cursor_position
        text = target_state.text
        if is_cursor_within_variable(cursor, text):
            variable_at_cursor = get_variable_at_cursor(cursor, text)
            return variable_at_cursor or ""
        else:
            return target_state.text

    def get_variable_candidates(self, target_state: TargetState) -> list[DropdownItem]:
        candidates = self.variable_candidates
        return candidates(target_state) if callable(candidates) else candidates
