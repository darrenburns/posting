from pathlib import Path
from textual.app import ComposeResult
from textual.containers import VerticalScroll
from textual.widget import Widget
from textual.widgets import Input, Label
from textual_autocomplete import AutoComplete, DropdownItem, TargetState

from posting.collection import Scripts


class RequestScripts(VerticalScroll):
    """Collections can contain a scripts folder.

    This widget is about linking scripts to requests.

    A script is a Python file which may contain functions
    named `pre_request` and/or `post_response`.

    Neither function is required, but if present and the path
    is supplied, Posting will automatically fetch the function
    from the file and execute it at the appropriate time.

    You can also specify the name of the function as a suffix
    after the path, separated by a colon.

    Example:
    ```
    scripts/on_request.py:prepare_auth
    scripts/on_response.py:log_response
    ```

    The API for scripts is under development and will likely change.

    The goal is to allow developers to attach scripts to requests,
    which will then be executed when the request is made, and when the
    response is received. This includes performing assertions, using
    plain assert statements, and hooks for deeper integration with Posting.
    For example, sending a notification when a request fails, or logging
    the response to a file.
    """

    DEFAULT_CSS = """
    RequestScripts {
        padding: 0 2;
        & Input {
            margin-bottom: 1;
        }
    }
    """

    def __init__(
        self,
        *children: Widget,
        collection_root: Path,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(
            *children, name=name, id=id, classes=classes, disabled=disabled
        )
        self.collection_root = collection_root

    def compose(self) -> ComposeResult:
        self.can_focus = False

        yield Label("Pre-request script [dim]optional[/dim]")
        yield Input(
            placeholder="Collection-relative path to pre-request script",
            id="pre-request-script",
        )

        yield Label("Post-response script [dim]optional[/dim]")
        yield Input(
            placeholder="Collection-relative path to post-response script",
            id="post-response-script",
        )

    def on_mount(self) -> None:
        auto_complete_pre_request = AutoComplete(
            candidates=self.get_script_candidates,
            target=self.query_one("#pre-request-script", Input),
        )
        auto_complete_post_response = AutoComplete(
            candidates=self.get_script_candidates,
            target=self.query_one("#post-response-script", Input),
        )

        self.screen.mount(auto_complete_pre_request)
        self.screen.mount(auto_complete_post_response)

    def get_script_candidates(self, state: TargetState) -> list[DropdownItem]:
        scripts: list[DropdownItem] = []
        for script in self.collection_root.glob("**/*.py"):
            scripts.append(DropdownItem(str(script.relative_to(self.collection_root))))
        return scripts

    def load_scripts(self, scripts: Scripts) -> None:
        self.query_one("#pre-request-script", Input).value = scripts.on_request or ""
        self.query_one("#post-response-script", Input).value = scripts.on_response or ""

    def to_model(self) -> Scripts:
        return Scripts(
            on_request=self.query_one("#pre-request-script", Input).value or None,
            on_response=self.query_one("#post-response-script", Input).value or None,
        )
