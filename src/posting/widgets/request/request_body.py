from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical
from textual.widgets import ContentSwitcher, Label
from posting.help_data import HelpData

from posting.widgets.center_middle import CenterMiddle
from posting.widgets.request.form_editor import FormEditor
from posting.widgets.select import PostingSelect
from posting.widgets.text_area import TextAreaFooter, TextEditor
from posting.widgets.variable_text_area import VariableTextArea


class RequestBodyEditor(Vertical):
    """
    A container for the request body text area and the request body type selector.
    """

    def compose(self) -> ComposeResult:
        with Horizontal(id="request-body-type-select-container"):
            yield PostingSelect(
                # These values are also referred to inside MainScreen.
                # When we load a request, we need to set the correct
                # value in the select.
                options=[
                    ("None", "no-body-label"),
                    ("Raw (json, text, etc.)", "text-body-editor"),
                    ("Form data (x-www-form-urlencoded)", "form-body-editor"),
                ],
                id="request-body-type-select",
                allow_blank=False,
            )
        with ContentSwitcher(
            initial="no-body-label",
            id="request-body-type-content-switcher",
        ):
            yield CenterMiddle(
                Label("No request body"),
                id="no-body-label",
            )
            text_area = RequestBodyTextArea(language="json")
            yield TextEditor(
                text_area=text_area,
                footer=TextAreaFooter(text_area),
                id="text-body-editor",
            )
            yield FormEditor(
                id="form-body-editor",
            )


class RequestBodyTextArea(VariableTextArea):
    """
    For editing request bodies.
    """

    BINDING_GROUP_TITLE = "Request Body Text Area"

    help = HelpData(
        title="Request Body Text Area",
        description="""\
A text area for entering the request body.
Type `$` to complete environment variables. Use `up` / `down` to choose,
`enter` / `tab` to insert, or `escape` to dismiss the dropdown.

Hold `shift` and move the cursor or click and drag to select text.
""",
    )

    def on_mount(self):
        self.tab_behavior = "indent"
        self.show_line_numbers = True
