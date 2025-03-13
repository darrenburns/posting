from typing import TYPE_CHECKING, Any, cast
from textual import on
from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical
from textual.lazy import Lazy
from textual.widgets import ContentSwitcher, Label, Select, TabPane
from posting.collection import RequestBody
from posting.widgets.center_middle import CenterMiddle
from posting.widgets.request.form_editor import FormEditor

from posting.widgets.request.header_editor import HeaderEditor
from posting.widgets.request.query_editor import QueryStringEditor
from posting.widgets.request.request_auth import RequestAuth
from posting.widgets.request.request_body import RequestBodyEditor, RequestBodyTextArea
from posting.widgets.request.request_metadata import RequestMetadata
from posting.widgets.request.request_options import RequestOptions
from posting.widgets.request.request_scripts import RequestScripts
from posting.widgets.select import PostingSelect
from posting.widgets.tabbed_content import PostingTabbedContent
from posting.widgets.text_area import TextAreaFooter, TextEditor


if TYPE_CHECKING:
    from posting.app import Posting


class RequestEditorTabbedContent(PostingTabbedContent):
    pass


class RequestEditor(Vertical):
    """
    The request editor.
    """

    def compose(self) -> ComposeResult:
        app = cast("Posting", self.app)
        with Vertical() as vertical:
            vertical.border_title = "Request"
            with RequestEditorTabbedContent():
                with TabPane("Headers", id="headers-pane"):
                    yield HeaderEditor()
                with TabPane("Body", id="body-pane"):
                    yield Lazy(RequestBodyEditor())
                with TabPane("Query", id="query-pane"):
                    yield Lazy(QueryStringEditor())
                with TabPane("Auth", id="auth-pane"):
                    yield Lazy(RequestAuth())
                with TabPane("Info", id="info-pane"):
                    yield Lazy(RequestMetadata())
                with TabPane("Scripts", id="scripts-pane"):
                    yield Lazy(RequestScripts(collection_root=app.collection.path))
                with TabPane("Options", id="options-pane"):
                    yield Lazy(RequestOptions())

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

    @property
    def query_editor(self) -> QueryStringEditor:
        return self.query_one(QueryStringEditor)

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
            # We can look at the language to determine the content type.
            return {
                "body": RequestBody(
                    content=text_editor.text,
                    content_type=text_editor.content_type,
                )
            }
        elif current == "form-body-editor":
            return {
                "body": RequestBody(
                    form_data=self.form_editor.to_model(),
                    content_type="application/x-www-form-urlencoded",
                )
            }
        return {}
