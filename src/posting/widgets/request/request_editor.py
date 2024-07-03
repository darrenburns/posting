from typing import Any
from textual import on
from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical
from textual.widgets import ContentSwitcher, Label, Select, TabPane
from posting.collection import RequestBody
from posting.widgets.center_middle import CenterMiddle
from posting.widgets.request.form_editor import FormEditor

from posting.widgets.request.header_editor import HeaderEditor
from posting.widgets.request.query_editor import QueryStringEditor
from posting.widgets.request.request_auth import RequestAuth
from posting.widgets.request.request_body import RequestBodyTextArea
from posting.widgets.request.request_metadata import RequestMetadata
from posting.widgets.request.request_options import RequestOptions
from posting.widgets.tabbed_content import PostingTabbedContent
from posting.widgets.text_area import TextAreaFooter, TextEditor


class RequestEditorTabbedContent(PostingTabbedContent):
    pass


class RequestEditor(Vertical):
    """
    The request editor.
    """

    DEFAULT_CSS = """\
    RequestEditor {
        & TextEditor {
            height: 1fr;
        }
        & #request-body-type-select-container {
            dock: top;
            height: 1;
        }
        & #no-body-label {
            height: 1fr;
            hatch: right $surface-lighten-1 70%;
        }
    }
"""

    def compose(self) -> ComposeResult:
        with Vertical() as vertical:
            vertical.border_title = "Request"
            with RequestEditorTabbedContent():
                with TabPane("Headers", id="headers-pane"):
                    yield HeaderEditor()
                with TabPane("Body", id="body-pane"):
                    with Horizontal(id="request-body-type-select-container"):
                        yield Select(
                            # These values are also referred to inside MainScreen.
                            # When we load a request, we need to set the correct
                            # value in the select.
                            options=[
                                ("None", "no-body-label"),
                                ("Raw (json, text, etc.)", "text-body-editor"),
                                ("Form data", "form-body-editor"),
                            ],
                            id="request-body-type-select",
                            allow_blank=False,
                        )
                    with ContentSwitcher(
                        initial="no-body-label",
                        id="request-body-type-content-switcher",
                    ):
                        yield CenterMiddle(
                            Label("The request doesn't have a body."),
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
                with TabPane("Parameters", id="parameters-pane"):
                    yield QueryStringEditor()
                with TabPane("Auth", id="auth-pane"):
                    yield RequestAuth()
                with TabPane("Metadata", id="metadata-pane"):
                    yield RequestMetadata()
                with TabPane("Options", id="options-pane"):
                    yield RequestOptions()

    def on_mount(self):
        self.border_title = "Request"
        self.add_class("section")

    @on(Select.Changed, selector="#request-body-type-select")
    def request_body_type_changed(self, event: Select.Changed) -> None:
        content_switcher = self.request_body_content_switcher
        content_switcher.current = event.value

    @property
    def request_body_type_select(self) -> Select[str]:
        return self.query_one("#request-body-type-select", Select)

    @property
    def request_body_content_switcher(self) -> ContentSwitcher:
        return self.query_one("#request-body-type-content-switcher", ContentSwitcher)

    @property
    def text_editor(self) -> TextEditor:
        return self.query_one("#text-body-editor", TextEditor)

    @property
    def form_editor(self) -> FormEditor:
        return self.query_one("#form-body-editor", FormEditor)

    def to_request_model_args(self) -> dict[str, Any]:
        """Returns a dictionary containing the arguments that should be
        passed to the httpx.Request object. The keys will depend on the
        content type that the user has selected."""
        content_switcher = self.request_body_content_switcher
        current = content_switcher.current
        text_editor = self.text_editor
        if current == "no-body-label":
            return {"body": None}
        elif current == "text-body-editor":
            # We need to check the chosen content type in the TextEditor
            return {"body": RequestBody(content=text_editor.text)}
        elif current == "form-body-editor":
            return {"body": RequestBody(form_data=self.form_editor.to_model())}
        return {}
