from dataclasses import dataclass
from pathlib import Path
from textual.app import ComposeResult
from textual.containers import VerticalScroll
from textual.screen import ModalScreen
from textual.widgets import Input, Label

from posting.collection import RequestModel


@dataclass
class SaveRequestModalData:
    """Data for the save request modal."""

    path: Path
    """The path to save the request to."""


class SaveRequestModal(ModalScreen[SaveRequestModalData]):
    """A modal for saving a request to disk if it has not already been saved.

    (Can also be used in situations where we're saving a copy of an existing request
    and thus want to change its name).
    """

    CSS = """
    SaveRequestModal {
        & Input {
            width: 100%;
        }

        & VerticalScroll {
            width: 30%;
            height: 70%;
        }
    }
    """

    def __init__(self, request: RequestModel):
        super().__init__()
        self.request = request
        """The request to save."""

    def compose(self) -> ComposeResult:
        with VerticalScroll():
            yield Input(placeholder="Title")
