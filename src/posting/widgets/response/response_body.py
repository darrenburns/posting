from dataclasses import dataclass
from textual import on
from textual.reactive import Reactive, reactive
from textual.app import ComposeResult
from textual.containers import Horizontal
from textual.widgets import Checkbox, Select, TextArea

from posting.text_area_theme import POSTLING_THEME


class ResponseBodyConfig(Horizontal):
    """The bar that appears above the response body, allowing
    you to customise the syntax highlighting, wrapping, line numbers,
    etc.
    """

    DEFAULT_CSS = """\
    ResponseBodyConfig {
        dock: bottom;
        height: 1;
        width: 1fr;
        background: $primary 10%;
        
        &:focus-within {
            background: $primary 55%;
        }

        &:disabled {
            background: transparent;
        }
        
        & Select {
            width: 8;
            & SelectCurrent {
                width: 8;
            }
            & SelectOverlay {
                width: 16;
            }
        }
        
        & Checkbox {
            margin-left: 1;
        }
    }
    """

    language: Reactive[str] = reactive("json", init=False)
    soft_wrap: Reactive[bool] = reactive(True, init=False)

    def compose(self) -> ComposeResult:
        with Horizontal(classes="dock-left w-auto"):
            yield Select(
                prompt="Content type",
                value=self.language,
                allow_blank=False,
                options=[("JSON", "json"), ("HTML", "html")],
                id="response-content-type-select",
            ).data_bind(value=ResponseBodyConfig.language)
            yield Checkbox(
                label="Wrap",
                value=self.soft_wrap,
                button_first=False,
                id="response-wrap-checkbox",
            ).data_bind(value=ResponseBodyConfig.soft_wrap)

    @on(Select.Changed, selector="#response-content-type-select")
    def update_language(self, event: Select.Changed) -> None:
        event.stop()
        self.language = event.value

    @on(Checkbox.Changed, selector="#response-wrap-checkbox")
    def update_soft_wrap(self, event: Checkbox.Changed) -> None:
        event.stop()
        self.soft_wrap = event.value


class ResponseTextArea(TextArea):
    """
    For displaying responses.
    """

    DEFAULT_CSS = """\
    ResponseTextArea {
        border: none;
        padding: 0;
        &:focus {
            border: none;
            padding: 0;
        }
    }
    """

    def on_mount(self):
        self.register_theme(POSTLING_THEME)
        self.theme = "posting"
        empty = len(self.text) == 0
        self.set_class(empty, "empty")
        self.show_line_numbers = not empty

    @on(TextArea.Changed)
    def on_change(self, event: TextArea.Changed) -> None:
        empty = len(self.text) == 0
        self.set_class(empty, "empty")
        self.show_line_numbers = not empty
