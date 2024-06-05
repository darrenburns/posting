from typing import TYPE_CHECKING
from textual import events
from textual.app import ComposeResult
from textual.geometry import Offset
from textual.screen import ModalScreen
from textual.widget import Widget
from textual.widgets import Label

if TYPE_CHECKING:
    from posting.jumper import JumpInfo


class JumpOverlay(ModalScreen[str | Widget]):
    """Overlay showing the jump targets.
    Returns the ID of the widget the jump was requested for on closing."""

    DEFAULT_CSS = """\
    JumpOverlay {
        background: black 25%;
    }
    """

    def __init__(
        self,
        overlays: dict[Offset, "JumpInfo"],
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
    ) -> None:
        super().__init__(name=name, id=id, classes=classes)
        self.overlays = overlays
        self.keys_to_widgets = {v.key: v.widget for v in overlays.values()}

    def on_key(self, key: events.Key) -> None:
        # Close the overlay if the user presses escape
        if key.key == "escape" or key.key == "ctrl+o":
            self.dismiss()
            return

        # If they press a key corresponding to a jump target,
        # then we jump to it.
        target = self.keys_to_widgets.get(key.key)
        if target is not None:
            self.dismiss(target)
            return

    def compose(self) -> ComposeResult:
        overlays = self.overlays
        for offset, jump_info in overlays.items():
            key, _widget = jump_info
            label = Label(key, classes="textual-jump-label")
            label.styles.offset = offset
            yield label
        yield Label("Press a key to jump", id="textual-jump-info")
