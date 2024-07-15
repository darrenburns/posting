from dataclasses import dataclass, field
from typing import Protocol, runtime_checkable
from rich.text import Text
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import VerticalScroll
from textual.screen import ModalScreen
from textual.widget import Widget
from textual.widgets import DataTable, Label, Markdown

from posting.widgets.datatable import PostingDataTable


@dataclass
class HelpData:
    """Data relating to the widget to be displayed in the HelpScreen"""

    title: str = field(default="")
    """Title of the widget"""
    description: str = field(default="")
    """Markdown description to be displayed in the HelpScreen"""


@runtime_checkable
class Helpable(Protocol):
    """Widgets which contain information to be displayed in the HelpScreen
    should implement this protocol."""

    HELP: HelpData


class HelpModalHeader(Label):
    """The top help bar"""

    DEFAULT_CSS = """
    HelpModalHeader {
        background: $background-lighten-1;
        color: $text-muted;
    }
    """


class HelpModalFooter(Label):
    """The bottom help bar"""

    DEFAULT_CSS = """
    HelpModalFooter {
        background: $background-lighten-1;
        color: $text-muted;
    }
    """


class HelpScreen(ModalScreen[None]):
    DEFAULT_CSS = """
    HelpScreen {
        align: center middle;
        & > VerticalScroll {
            background: $background;
            padding: 1 2;
            width: 50%;
            height: 70%;
            border: wide $background-lighten-2;
            border-title-color: $text;
            border-title-background: $background;
            border-title-style: bold;
        }

        & DataTable#bindings-table {
            width: 1fr;
            height: 1fr;
        }

        & HelpModalHeader {
            dock: top;
            width: 1fr;
            content-align: center middle;
        }
        & HelpModalFooter {
            dock: bottom;
            width: 1fr;
            content-align: center middle;
        }

        & #bindings-title {
            width: 1fr;
            content-align: center middle;
            background: $background-lighten-1;
            color: $text-muted;
        }

        & #help-description {
            dock: top;
            margin-top: 1;
            width: 1fr;
        }
    }
    """

    BINDINGS = [
        Binding("escape", "app.pop_screen", "Close Help"),
    ]

    def __init__(
        self,
        widget: Widget,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
    ) -> None:
        super().__init__(name, id, classes)
        self.widget = widget

    def compose(self) -> ComposeResult:
        with VerticalScroll() as vs:
            vs.can_focus = False
            widget = self.widget
            bindings = widget._bindings
            keys: list[tuple[str, Binding]] = [
                binding for binding in bindings.keys.items()
            ]

            if keys:
                yield Label(" [b]Keybindings[/]", id="bindings-title")
                table = PostingDataTable(
                    id="bindings-table",
                    cursor_type="row",
                    zebra_stripes=True,
                )
                table.add_columns("Key", "Description")
                for key, binding in keys:
                    table.add_row(Text(key, style="bold"), binding.description)
                yield table

            # If the widget has help text, render it.
            if isinstance(widget, Helpable):
                help_title = widget.HELP.title
                if help_title:
                    yield HelpModalHeader(f"[b]{help_title}[/] Help")
                help_markdown = widget.HELP.description

                if help_markdown:
                    yield Markdown(help_markdown, id="help-description")
                else:
                    yield Label(
                        f"No help available for {widget.HELP.title}",
                        id="help-description",
                    )
            else:
                yield HelpModalHeader(f"[b]{widget.__class__.__name__}[/] Help")

            yield HelpModalFooter("Press ESC to close help.")
