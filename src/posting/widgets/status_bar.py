from rich.text import Text
from textual.app import ComposeResult
from textual.containers import Horizontal
from textual.reactive import Reactive
from textual.widgets import Label

from posting.collection import RequestModel

NO_REQUEST_SELECTED = "Unsaved request"


class FileStatus(Label):
    """The label containing the open file name."""

    dirty: Reactive[bool] = Reactive(False)

    def render(self) -> Text:
        return Text(
            f"{self.renderable}",
            style="italic" if self.dirty else "",
            end="",
            overflow="ellipsis",
            no_wrap=True,
        )


class StatusBar(Horizontal):
    """A status bar at the bottom of the screen."""

    dirty: Reactive[bool] = Reactive(False)

    def compose(self) -> ComposeResult:
        yield FileStatus(NO_REQUEST_SELECTED).data_bind(StatusBar.dirty)

    def set_request_status(self, request: RequestModel | None) -> None:
        status = self.query_one(FileStatus)
        if request is None:
            status.update(NO_REQUEST_SELECTED)
            self.dirty = False
        else:
            status.update(
                Text(
                    request.path.name,
                    overflow="ellipsis",
                    no_wrap=True,
                    end="",
                )
            )
            self.dirty = True
