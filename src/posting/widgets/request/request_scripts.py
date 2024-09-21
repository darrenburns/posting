from textual.app import ComposeResult
from textual.containers import Vertical, VerticalScroll
from textual.widgets import Input


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
    scripts/pre_request.py:prepare_auth
    scripts/post_response.py:log_response
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
        & > #pre-request-script {
            background: $panel;
        }
        
        & > #post-response-script {
            background: $panel;
        }
    }
    """

    def compose(self) -> ComposeResult:
        with Vertical(classes="section"):
            yield Input(placeholder="Pre-request Script")

        with Vertical(classes="section"):
            yield Input(placeholder="Post-response Script")
