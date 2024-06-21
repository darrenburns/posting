from textual import on
from textual.widgets import TextArea

from posting.widgets.text_area import POSTING_THEME, ReadOnlyTextArea


class ResponseTextArea(ReadOnlyTextArea):
    """
    For displaying responses.
    """

    @on(TextArea.Changed)
    def on_change(self, event: TextArea.Changed) -> None:
        empty = len(self.text) == 0
        self.set_class(empty, "empty")
        self.show_line_numbers = not empty
