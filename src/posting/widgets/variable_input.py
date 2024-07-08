from textual.widgets import Input
from textual_autocomplete import DropdownItem, TargetState
from posting.highlighters import VariableHighlighter
from posting.variables import get_variables

from posting.widgets.variable_autocomplete import VariableAutoComplete


class VariableInput(Input):
    def on_mount(self) -> None:
        self.highlighter = VariableHighlighter()
        self.auto_complete = VariableAutoComplete(
            candidates=[],
            variable_candidates=self._get_variable_candidates,
            target=self,
        )
        self.screen.mount(self.auto_complete)

    def _get_variable_candidates(self, target_state: TargetState) -> list[DropdownItem]:
        return [DropdownItem(main=f"${variable}") for variable in get_variables()]
