"""Schema-aware autocompletion for the GraphQL query editor."""

from __future__ import annotations

from typing import Callable

from textual import events
from textual.content import Content
from textual.widgets import TextArea
from textual_autocomplete import AutoComplete, DropdownItem, TargetState

from posting.graphql.completion import Completion, Context, analyze, complete
from posting.graphql.schema import Schema

KIND_PREFIXES: dict[str, str] = {
    "argument": "arg ",
    "enum": "enum ",
    "type": "type ",
    "variable": "$",
}
"""A hint shown to the left of a candidate, so it's clear what's being completed."""


class GraphQLAutoComplete(AutoComplete):
    """A dropdown of GraphQL completions for a `TextArea`.

    `AutoComplete` targets `Input` widgets, which differ from text areas in two
    ways that matter here: the cursor is a row/column pair rather than an
    offset, and a text area handles keys such as `enter` before the dropdown
    gets a chance to. Key handling therefore lives in the text area itself
    (see `GraphQLQueryTextArea`), which drives this widget through
    `highlight_next`, `highlight_previous`, `accept` and `open`.
    """

    def __init__(
        self,
        target: TextArea,
        schema_provider: Callable[[], Schema | None],
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        """
        Args:
            target: The text area to complete within.
            schema_provider: Returns the schema to complete against, or `None`
                if no schema has been fetched for the current endpoint.
        """
        super().__init__(
            target=target,  # type: ignore[arg-type]
            candidates=None,
            name=name,
            id=id,
            classes=classes,
            disabled=disabled,
        )
        self.schema_provider = schema_provider
        self._context_cache: tuple[str, int, Context] | None = None
        self._completed_text: str | None = None
        """The text of the target immediately after applying a completion."""

    @property
    def target(self) -> TextArea:  # type: ignore[override]
        """The text area being completed within."""
        if isinstance(self._target, TextArea):
            return self._target
        target = self.screen.query_one(self._target)
        assert isinstance(target, TextArea)
        return target

    @property
    def is_open(self) -> bool:
        """Whether the dropdown is currently visible."""
        return self.display and self.option_list.option_count > 0

    def _listen_to_messages(self, event: events.Event) -> None:
        """Rebuild the dropdown as the user types.

        Unlike the base class we ignore key events - a text area acts on them
        before they reach us, so they're handled by the text area instead.
        """
        if not isinstance(event, TextArea.Changed):
            return
        if self._completed_text is not None:
            # This is the change we caused by applying a completion, so the
            # dropdown should stay closed rather than immediately reopening.
            if self.target.text == self._completed_text:
                self._completed_text = None
                return
            self._completed_text = None
        self._handle_target_update()

    def _get_target_state(self) -> TargetState:
        target = self.target
        return TargetState(
            text=target.text,
            cursor_position=target.document.get_index_from_location(
                target.cursor_location
            ),
        )

    def _analysis(self, target_state: TargetState) -> Context:
        """The (cached) analysis of the document at the cursor."""
        text, cursor = target_state.text, target_state.cursor_position
        if self._context_cache is not None:
            cached_text, cached_cursor, context = self._context_cache
            if cached_text == text and cached_cursor == cursor:
                return context
        context = analyze(text, cursor)
        self._context_cache = (text, cursor, context)
        return context

    def get_search_string(self, target_state: TargetState) -> str:
        return self._analysis(target_state).prefix

    def get_candidates(self, target_state: TargetState) -> list[DropdownItem]:
        schema = self.schema_provider()
        if schema is None:
            return []
        completions = complete(schema, target_state.text, target_state.cursor_position)
        return [_dropdown_item(completion) for completion in completions]

    def apply_completion(self, value: str, state: TargetState) -> None:
        """Replace the partially typed word at the cursor with `value`."""
        target = self.target
        document = target.document
        context = self._analysis(state)
        start = document.get_location_from_index(context.start)
        end = document.get_location_from_index(state.cursor_position)
        target.replace(value, start, end, maintain_selection_offset=False)
        self._completed_text = target.text
        self._context_cache = None

    def post_completion(self) -> None:
        self.action_hide()

    def highlight_next(self) -> None:
        """Highlight the next candidate in the dropdown."""
        self._move_highlight(1)

    def highlight_previous(self) -> None:
        """Highlight the previous candidate in the dropdown."""
        self._move_highlight(-1)

    def _move_highlight(self, delta: int) -> None:
        option_list = self.option_list
        if not option_list.option_count:
            return
        highlighted = option_list.highlighted
        option_list.highlighted = (
            0
            if highlighted is None
            else (highlighted + delta) % option_list.option_count
        )

    def accept(self) -> None:
        """Insert the highlighted candidate into the text area."""
        self._complete(self.option_list.highlighted or 0)

    def open(self) -> None:
        """Show the dropdown for the current cursor position, if we have candidates."""
        target_state = self._get_target_state()
        self._rebuild_options(target_state, self.get_search_string(target_state))
        if self.option_list.option_count:
            self.action_show()


def _dropdown_item(completion: Completion) -> DropdownItem:
    prefix = KIND_PREFIXES.get(completion.kind)
    return DropdownItem(
        main=completion.name,
        prefix=Content.styled(prefix, "dim") if prefix else None,
    )
