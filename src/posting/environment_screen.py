from rich.text import Text
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical, VerticalScroll
from textual.screen import ModalScreen
from textual.widget import Widget
from textual.widgets import Label, Input, Select
from textual.widgets.data_table import RowKey


from posting.widgets.datatable import PostingDataTable
from posting.widgets.key_value import KeyValueEditor, KeyValueInput
from posting.widgets.input import PostingInput
from posting.variables import (
    get_variables,
    update_variables,
    remove_variable,
    get_environments,
    set_current_environment,
    get_current_environment,
)
from posting.help_screen import HelpData


class KeyInput(PostingInput):
    help = HelpData(
        title="Key Input",
        description="""\
An input field for entering environment variable keys.
""",
    )


class ValueInput(PostingInput):
    help = HelpData(
        title="Value Input",
        description="""\
An input field for entering environment variable values.
""",
    )


class EnvironmentModalHeader(Label):
    """The top help bar"""

    DEFAULT_CSS = """
    EnvironmentModalHeader {
        background: $background-lighten-1;
        color: $text-muted;
    }
    """


class EnvironmentModalFooter(Label):
    """The bottom help bar"""

    DEFAULT_CSS = """
    EnvironmentModalFooter {
        background: $background-lighten-1;
        color: $text-muted;
    }
    """


class EnvironmentTable(PostingDataTable):
    """
    The environment table.
    """

    BINDINGS = [
        Binding("backspace", action="remove_row", description="Remove row"),
    ]

    def on_mount(self):
        self.fixed_columns = 1
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.add_columns(*["Key", "Value"])
        self.replace_all_rows(get_variables().items())

    def watch_has_focus(self, value: bool) -> None:
        self._scroll_cursor_into_view()
        return super().watch_has_focus(value)

    def add_row(
        self,
        *cells: str | Text,
        height: int | None = 1,
        key: str | None = None,
        label: str | Text | None = None,
        explicit_by_user: bool = True,
    ) -> RowKey:
        update_variables({cells[0]: cells[1]})
        return super().add_row(
            *cells,
            height=height,
            key=cells[0],
            label=label,
            explicit_by_user=explicit_by_user,
        )

    def remove_row(self, row_key: RowKey | str) -> None:
        key = self.get_row(row_key)[0]
        remove_variable(key)
        return super().remove_row(row_key)


class EnvironmentScreen(ModalScreen[None]):
    DEFAULT_CSS = """
    EnvironmentScreen {
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

        & EnvironmentModalHeader {
            dock: top;
            width: 1fr;
            content-align: center middle;
        }

        #footer-area {
            dock: bottom;
            height: auto;
            margin-top: 1;
            & EnvironmentModalFocusNote {
                width: 1fr;
                content-align: center middle;
                color: $text-muted 40%;
            }

            & EnvironmentModalFooter {
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
        Binding("escape", "dismiss('')", "Close Environment"),
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
        
    def on_mount(self) -> None:
        self.query_one("#environment-table", EnvironmentTable).focus()

    def compose(self) -> ComposeResult:
        with VerticalScroll() as vs:
            yield EnvironmentModalHeader("[b]Edit Environment Variables[/]")

            yield Select(
                [(env.name, env) for env in get_environments()],
                allow_blank=False,
                value=get_current_environment(),
            )

            yield KeyValueEditor(
                EnvironmentTable(id="environment-table"),
                KeyValueInput(
                    KeyInput(placeholder="Key", id="environment-key-input"),
                    ValueInput(placeholder="Value", id="environment-value-input"),
                    button_label="Add",
                ),
                empty_message="There are no environment variables.",
            )

            with Vertical(id="footer-area"):
                yield EnvironmentModalFooter("Press [b]ESC[/] to dismiss.")

    def on_select_changed(self, changed: Select.Changed) -> None:
        set_current_environment(changed.value)
        self.query_one("#environment-key-input", Input).value = ""
        self.query_one("#environment-value-input", Input).value = ""
        env_table = self.query_one("#environment-table", EnvironmentTable)
        env_table.replace_all_rows(get_variables().items())
        env_table.focus()
