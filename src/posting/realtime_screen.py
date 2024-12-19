from pathlib import Path

from textual.app import ComposeResult
from textual.binding import Binding
from textual.screen import Screen
from textual.widgets import Footer, Label

from posting.collection import Collection
from posting.config import SETTINGS
from posting.http_screen import AppBody, AppHeader
from posting.widgets.collection.browser import CollectionBrowser, CollectionTree
from posting.widgets.request.url_bar import UrlBar


class RealtimeScreen(Screen):
    BINDINGS = [
        Binding(
            "ctrl+h",
            "toggle_collection_browser",
            "Toggle collection browser",
            show=False,
            tooltip="Toggle the collection browser.",
            id="toggle-collection",
        ),
    ]

    def __init__(self, collection: Collection, environment_files: list[Path]):
        super().__init__()
        self.collection = collection
        self.environment_files = environment_files
        self.settings = SETTINGS.get()

    def compose(self) -> ComposeResult:
        yield AppHeader()
        yield UrlBar(mode="realtime")
        with AppBody():
            collection_browser = CollectionBrowser(
                mode="realtime", collection=self.collection
            )
            collection_browser.display = (
                self.settings.collection_browser.show_on_startup
            )
            yield collection_browser
            yield Label("Realtime")
        yield Footer(show_command_palette=False)

    def action_toggle_collection_browser(self) -> None:
        """Toggle the collection browser."""
        collection_browser = self.collection_browser
        collection_browser.display = not collection_browser.display

    @property
    def collection_browser(self) -> CollectionBrowser:
        return self.query_one(CollectionBrowser)
