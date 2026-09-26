"""Variable completion for editable request text, using textual-autocomplete."""

import re

from textual import events
from textual.message import Message
from textual.widgets import TextArea
from textual_autocomplete import DropdownItem, TargetState

from posting.variables import get_variables
from posting.widgets.text_area import PostingTextArea
from posting.widgets.variable_autocomplete import VariableAutoComplete


class TextAreaVariableAutoComplete(VariableAutoComplete):
    @property
    def target(self) -> TextArea:
        target = self._target
        assert isinstance(target, TextArea)
        return target

    def _get_target_state(self) -> TargetState:
        target = self.target
        return TargetState(
            target.text, target.document.get_index_from_location(target.cursor_location)
        )

    def _variable_range(self, state: TargetState) -> tuple[int, int] | None:
        # Match an unfinished variable immediately before the cursor. Paired dollars
        # are literal escapes, and braces may still be empty or incomplete.
        match = re.search(
            r"(?:^|[^$])(?:\$\$)*(\$\{?[a-zA-Z_0-9]*)$",
            state.text[: state.cursor_position],
        )
        if match is None or not self.target.selection.is_empty:
            return None
        start = match.start(1)
        suffix = re.match(r"\w*", state.text[state.cursor_position :]).group()
        end = state.cursor_position + len(suffix)
        if state.text[start : start + 2] == "${" and state.text[end : end + 1] == "}":
            end += 1
        return start, end

    def get_candidates(self, state: TargetState) -> list[DropdownItem]:
        return (
            [DropdownItem(f"${name}") for name in get_variables()]
            if self._variable_range(state)
            else []
        )

    def get_search_string(self, state: TargetState) -> str:
        span = self._variable_range(state)
        return (
            state.text[span[0] : state.cursor_position].replace("{", "") if span else ""
        )

    def apply_completion(self, value: str, state: TargetState) -> None:
        span = self._variable_range(state)
        if span is None:
            return
        start, end = span
        if state.text[start : start + 2] == "${":
            value = "${" + value[1:] + "}"
        target = self.target
        target.history.checkpoint()
        target.replace(
            value,
            target.document.get_location_from_index(start),
            target.document.get_location_from_index(end),
        )
        target.history.checkpoint()
        self.post_completion()

    def should_show_dropdown(self, search_string: str) -> bool:
        options = self.option_list
        return bool(
            search_string
            and options.option_count
            and not (
                options.option_count == 1
                and options.get_option_at_index(0).value == search_string
            )
        )

    def _listen_to_messages(self, event: Message) -> None:
        # Keys must be intercepted before TextArea's insertion/indentation handlers.
        # The message signal runs after those handlers, so only observe edits here.
        if isinstance(event, TextArea.Changed):
            self._handle_target_update()

    def handle_key(self, event: events.Key) -> bool:
        if not self.display or not self.option_list.option_count:
            return False
        if event.key not in {"up", "down", "enter", "tab", "escape"}:
            return False
        super()._listen_to_messages(event)
        event.prevent_default()
        event.stop()
        return True


class VariableTextArea(PostingTextArea):
    def on_mount(self) -> None:
        self.auto_complete = TextAreaVariableAutoComplete(self, candidates=[])
        self.screen.mount(self.auto_complete)

    def on_key(self, event: events.Key) -> None:
        if self.auto_complete.handle_key(event):
            event.prevent_default()

    def on_unmount(self) -> None:
        self.auto_complete.remove()
