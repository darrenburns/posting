from textual.app import ComposeResult
from textual.containers import Vertical

from posting.widgets.text_area import TextEditor, PostingTextArea, TextAreaFooter


class MessageEditor(Vertical):
    def compose(self) -> ComposeResult:
        text_area = PostingTextArea(
            language="json", tab_behavior="indent", show_line_numbers=True
        )
        yield TextEditor(
            text_area=text_area, footer=TextAreaFooter(text_area=text_area)
        )
