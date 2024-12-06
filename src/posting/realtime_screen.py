from pathlib import Path

from textual.app import ComposeResult
from textual.screen import Screen
from textual.widgets import Label

from posting.collection import Collection


class RealtimeScreen(Screen):
    def __init__(self, collection: Collection, environment_files: list[Path]):
        super().__init__()
        self.collection = collection
        self.environment_files = environment_files

    def compose(self) -> ComposeResult:
        yield Label("Realtime")
