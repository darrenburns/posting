from textual import on
from textual.widgets import TextArea

from posting.widgets.text_area import POSTLING_THEME, ReadOnlyTextArea


class ResponseTextArea(ReadOnlyTextArea):
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
