from typing import Any, Mapping, NamedTuple, Protocol, runtime_checkable
from textual.errors import NoWidget

from textual.geometry import Offset
from textual.screen import Screen
from textual.widget import Widget


@runtime_checkable
class Jumpable(Protocol):
    """A widget which we can jump focus to."""

    jump_key: str


class JumpInfo(NamedTuple):
    """Information returned by the jumper for each jump target."""

    key: str
    """The key which should trigger the jump."""

    widget: str | Widget
    """Either the ID or a direct reference to the widget."""


class Jumper:
    """An Amp-like jumping mechanism for quick spatial navigation"""

    def __init__(self, ids_to_keys: Mapping[str, str], screen: Screen[Any]) -> None:
        self.ids_to_keys = ids_to_keys
        self.keys_to_ids = {v: k for k, v in ids_to_keys.items()}
        self.screen = screen

    def get_overlays(self) -> dict[Offset, JumpInfo]:
        """Return a dictionary of all the jump targets"""
        screen = self.screen
        children: list[Widget] = screen.walk_children(Widget)
        overlays: dict[Offset, JumpInfo] = {}
        ids_to_keys = self.ids_to_keys
        for child in children:
            try:
                widget_offset = screen.get_offset(child)
            except NoWidget:
                # The widget might not be visible in the layout
                # due to it being hidden in some modes.
                continue

            if child.id and child.id in ids_to_keys:
                overlays[widget_offset] = JumpInfo(
                    ids_to_keys[child.id],
                    child.id,
                )
            elif isinstance(child, Jumpable):
                overlays[widget_offset] = JumpInfo(
                    child.jump_key,
                    child,
                )

        return overlays
