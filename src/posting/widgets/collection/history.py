"""The collection sidebar's response history."""

import sqlite3
from dataclasses import dataclass
from typing import cast

import httpx
from rich.text import Text
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.message import Message
from textual.widgets import OptionList, Static
from textual.widgets.option_list import Option

from posting.config import SETTINGS
from posting.history import HistoryEntry, HistoryStore
from posting.widgets.confirmation import ConfirmationModal


class HistoryList(OptionList):
    BINDINGS = [
        Binding("j", "cursor_down", "Down", show=False),
        Binding("k", "cursor_up", "Up", show=False),
        Binding("g", "first", "First", show=False),
        Binding("G", "last", "Last", show=False),
        Binding("l", "select", "View response", show=False),
        Binding("backspace", "delete_entry", "Delete"),
        Binding("ctrl+backspace", "clear_history", "Clear history"),
    ]

    def action_delete_entry(self) -> None:
        cast(HistoryBrowser, self.parent).delete_selected()

    def action_clear_history(self) -> None:
        cast(HistoryBrowser, self.parent).confirm_clear()


class HistoryBrowser(Vertical):
    DEFAULT_CSS = """
    HistoryBrowser {
        height: 1fr;
        #history-empty {
            padding: 1 2;
            color: $text-muted;
        }
        HistoryList {
            height: 1fr;
            border: none;
            padding: 0;
            background: transparent;
            text-wrap: nowrap;
            text-overflow: ellipsis;
            &:focus { border: none; }
            & > .option-list--option {
                padding: 0 1 1 1;
            }
        }
        #history-detail {
            dock: bottom;
            height: 4;
            text-wrap: nowrap;
            text-overflow: ellipsis;
            padding: 0 1;
            border-top: solid $accent 40%;
            color: $text-muted;
        }
    }
    """

    @dataclass
    class Selected(Message):
        entry: HistoryEntry
        response: httpx.Response

    def __init__(self, store: HistoryStore) -> None:
        super().__init__()
        self.store = store
        self.entries: list[HistoryEntry] = []

    def compose(self) -> ComposeResult:
        yield Static(id="history-empty")
        yield HistoryList(id="history-list")
        yield Static(id="history-detail", markup=False)

    def on_mount(self) -> None:
        self.refresh_history()
        self.app.theme_changed_signal.subscribe(self, lambda _: self.refresh_history())

    def report_error(self, error: Exception) -> None:
        self.notify(str(error), title="History unavailable", severity="warning")

    def refresh_history(self) -> None:
        history_list = self.query_one(HistoryList)
        selected_id = None
        if history_list.highlighted is not None and self.entries:
            selected_id = self.entries[history_list.highlighted].id
        unavailable = False
        try:
            self.entries = self.store.entries()
        except (OSError, sqlite3.Error, ValueError) as error:
            self.report_error(error)
            self.entries = []
            unavailable = True
        history_list.clear_options()
        variables = self.app.theme_variables
        for entry in self.entries:
            color = (
                "text-success"
                if entry.status_code < 300
                else ("text-warning" if entry.status_code < 400 else "text-error")
            )
            timestamp = entry.received_at.astimezone().strftime("%d %b %H:%M")
            prompt = Text(no_wrap=True, overflow="ellipsis")
            prompt.append(f"{entry.method} ", style="bold")
            prompt.append(str(entry.status_code), style=variables[color])
            prompt.append(f" · {timestamp}\n", style="dim")
            prompt.append(entry.url.removeprefix("https://").removeprefix("http://"))
            history_list.add_option(Option(prompt, id=str(entry.id)))
        history_list.highlighted = next(
            (i for i, entry in enumerate(self.entries) if entry.id == selected_id),
            0 if self.entries else None,
        )
        history_list.display = bool(self.entries)
        empty = self.query_one("#history-empty", Static)
        empty.display = not self.entries
        empty.update(
            "[b]No responses yet[/b]\n\nSend a request to keep its response here.\n\nHistory is saved locally for this collection."
            if SETTINGS.get().history.enabled
            else "[b]History is paused[/b]\n\nEnable history in your configuration to save new responses."
        )
        if unavailable:
            empty.update(
                "[b]History unavailable[/b]\n\nThe local history could not be read. You can still send requests and view their responses."
            )
        self.update_detail()

    @on(OptionList.OptionHighlighted)
    def update_detail(self) -> None:
        history_list = self.query_one(HistoryList)
        detail = self.query_one("#history-detail", Static)
        index = history_list.highlighted
        detail.display = index is not None and bool(self.entries)
        if index is not None and index < len(self.entries):
            entry = self.entries[index]
            timestamp = entry.received_at.astimezone().strftime("%d %b %Y · %H:%M:%S")
            detail.tooltip = f"{entry.method} {entry.url}"
            detail.update(
                f"{entry.method} {entry.url}\n{timestamp}\nEnter to view response"
            )

    @on(OptionList.OptionSelected)
    def select_response(self, event: OptionList.OptionSelected) -> None:
        event.stop()
        entry = self.entries[event.option_index]
        try:
            response = self.store.load(entry.id)
        except (OSError, sqlite3.Error, ValueError, httpx.HTTPError) as error:
            self.report_error(error)
            return
        if response is None:
            self.refresh_history()
            self.notify("This response is no longer in history.")
            return
        self.post_message(self.Selected(entry, response))

    def delete_selected(self) -> None:
        index = self.query_one(HistoryList).highlighted
        if index is None or not self.entries:
            return
        try:
            self.store.delete(self.entries[index].id)
        except (OSError, sqlite3.Error) as error:
            self.report_error(error)
            return
        self.refresh_history()

    def confirm_clear(self) -> None:
        if not self.entries:
            return

        def clear(confirmed: bool) -> None:
            if confirmed:
                try:
                    self.store.clear()
                except (OSError, sqlite3.Error) as error:
                    self.report_error(error)
                    return
                self.refresh_history()

        self.app.push_screen(
            ConfirmationModal(
                "Clear all response history for this collection?",
                confirm_text="Clear \\[y]",
                auto_focus="cancel",
            ),
            clear,
        )
