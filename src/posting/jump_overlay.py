from typing import TYPE_CHECKING
from textual import events, log
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Center
from textual.screen import ModalScreen
from textual.widget import Widget
from textual.widgets import Label
import asyncio
import time

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
        self._last_resize_time: float = 0
        self._debounce_running: bool = False

    def on_key(self, key_event: events.Key) -> None:
        # We need to stop the bubbling of these keys, because if they
        # arrive at the parent after the overlay is closed, then the parent
        # will handle the key event, resulting in the focus being shifted
        # again (unexpectedly) after the jump target was focused.
        log.debug(f"Handling jump key press: {key_event.key}")
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

        # Update the last resize time
        self._last_resize_time = time.time()

        # Start the debounce task if it's not already running
        if not self._debounce_running:
            self._debounce_running = True
            asyncio.create_task(self._debounced_recompose())

    async def on_unmount(self) -> None:
        # Nothing to cancel since we're using a different approach
        self._debounce_running = False

    async def _debounced_recompose(self) -> None:
        try:
            while self._debounce_running:
                # Get the current time
                current_time = time.time()
                # Calculate time since last resize
                time_since_last_resize = current_time - self._last_resize_time

                if time_since_last_resize >= 0.05:
                    await self.recompose()
                    break

                # Otherwise, wait a bit and check again
                await asyncio.sleep(0.1)

            self._debounce_running = False
        except asyncio.CancelledError:
            self._debounce_running = False

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
