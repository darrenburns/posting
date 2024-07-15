from textual import on
from textual.widgets import TextArea
from posting.help_screen import HelpData

from posting.widgets.text_area import ReadOnlyTextArea


class ResponseTextArea(ReadOnlyTextArea):
    """
    For displaying responses.
    """

    help = HelpData(
        title="Response Body",
        description="""\
A *read-only* text area for displaying the response body.
Supports several Vim keys (see table below).
Open the response in your `$PAGER` by pressing `f3`. A custom pager (e.g. `fx`)
can be used for JSON responses by setting the `pager_json` config to the command.
""",
    )

    @on(TextArea.Changed)
    def on_change(self, event: TextArea.Changed) -> None:
        empty = len(self.text) == 0
        self.set_class(empty, "empty")
        self.show_line_numbers = not empty
