from typing import TYPE_CHECKING
from textual import events
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Center
from textual.screen import ModalScreen
from textual.widget import Widget
from textual.widgets import Label

if TYPE_CHECKING:
    from posting.jumper import Jumper


class JumpOverlay(ModalScreen[str | Widget | None]):
    """Overlay showing the jump targets.
    Dismissed with the ID of the widget the jump was requested for on closing,
    or a reference to the widget. Is dismissed with None if the user dismissed
    the overlay without making a selection."""

    DEFAULT_CSS = """\
    JumpOverlay {
        background: black 25%;
    }
    """

    BINDINGS = [
        Binding("escape", "dismiss_overlay", "Dismiss", show=False),
    ]

    def __init__(
        self,
        jumper: "Jumper",
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
    ) -> None:
        super().__init__(name=name, id=id, classes=classes)
        self.jumper: Jumper = jumper
        self.keys_to_widgets: dict[str, Widget | str] = {}
        self._resize_counter = 0

    def on_key(self, key_event: events.Key) -> None:
        # We need to stop the bubbling of these keys, because if they
        # arrive at the parent after the overlay is closed, then the parent
        # will handle the key event, resulting in the focus being shifted
        # again (unexpectedly) after the jump target was focused.
        if key_event.key == "tab" or key_event.key == "shift+tab":
            key_event.stop()
            key_event.prevent_default()

        if self.is_active:
            # If they press a key corresponding to a jump target,
            # then we jump to it.
            target = self.keys_to_widgets.get(key_event.key)
            if target is not None:
                self.dismiss(target)
                return

    def action_dismiss_overlay(self) -> None:
        self.dismiss(None)

    async def on_resize(self) -> None:
        self._resize_counter += 1
        if self._resize_counter == 1:
            return
        await self.recompose()

    def _sync(self) -> None:
        self.overlays = self.jumper.get_overlays()
        self.keys_to_widgets = {v.key: v.widget for v in self.overlays.values()}

    def compose(self) -> ComposeResult:
        self._sync()
        for offset, jump_info in self.overlays.items():
            key, _widget = jump_info
            label = Label(key, classes="textual-jump-label")
            x, y = offset
            label.styles.margin = y, x
            yield label
        with Center(id="textual-jump-info"):
            yield Label("Press a key to jump")
        with Center(id="textual-jump-dismiss"):
            yield Label("[b]ESC[/] to dismiss")
