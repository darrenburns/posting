from dataclasses import dataclass, field
from typing import Protocol, runtime_checkable
from rich.text import Text
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical, VerticalScroll
from textual.screen import ModalScreen
from textual.widget import Widget
from textual.widgets import Label, Markdown

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

    help: HelpData


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


class HelpModalFocusNote(Label):
    """A note below the help screen."""


class HelpScreen(ModalScreen[None]):
    DEFAULT_CSS = """
    HelpScreen {
        align: center middle;
        & > VerticalScroll {
            background: $background;
            padding: 1 2;
            width: 65%;
            height: 80%;
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

        #footer-area {
            dock: bottom;
            height: auto;
            margin-top: 1;
            & HelpModalFocusNote {
                width: 1fr;
                content-align: center middle;
                color: $text-muted 40%;
            }

            & HelpModalFooter {
                width: 1fr;
                content-align: center middle;
            }
        }


        & #bindings-title {
            width: 1fr;
            content-align: center middle;
            background: $background-lighten-1;
            color: $text-muted;
        }

        & #help-description-wrapper {
            dock: top;
            max-height: 50%;
            margin-top: 1;
            height: auto;
            width: 1fr;
            & #help-description {
                margin: 0;
                width: 1fr;
                height: auto;
            }
        }
    }
    """

    BINDINGS = [
        Binding("escape", "dismiss('')", "Close Help"),
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
            # If the widget has help text, render it.
            if isinstance(widget, Helpable):
                help = widget.help
                help_title = help.title
                vs.border_title = f"[not bold]Focused Widget Help ([b]{help_title}[/])"
                if help_title:
                    yield HelpModalHeader(f"[b]{help_title}[/]")
                help_markdown = help.description

                if help_markdown:
                    help_markdown = help_markdown.strip()
                    with VerticalScroll(id="help-description-wrapper") as vs:
                        yield Markdown(help_markdown, id="help-description")
                else:
                    yield Label(
                        f"No help available for {help.title}",
                        id="help-description",
                    )
            else:
                name = widget.__class__.__name__
                vs.border_title = f"Focused Widget Help ([b]{name}[/])"
                yield HelpModalHeader(f"[b]{name}[/] Help")

            bindings = widget._bindings
            keys: list[tuple[str, list[Binding]]] = list(
                bindings.key_to_bindings.items()
            )

            if keys:
                yield Label(" [b]All Keybindings[/]", id="bindings-title")
                table = PostingDataTable(
                    id="bindings-table",
                    cursor_type="row",
                    zebra_stripes=True,
                )
                table.cursor_vertical_escape = False
                table.add_columns("Key", "Description")
                for key, bindings in keys:
                    table.add_row(
                        Text(
                            ", ".join(
                                binding.key_display
                                if binding.key_display
                                else self.app.get_key_display(binding)
                                for binding in bindings
                            ),
                            style="bold",
                            no_wrap=True,
                            end="",
                        ),
                        bindings[0].description.lower(),
                    )
                yield table

            with Vertical(id="footer-area"):
                yield HelpModalFooter("Press [b]ESC[/] to dismiss.")
                yield HelpModalFocusNote(
                    "[b]Note:[/] This page relates to the widget that is currently focused."
                )
