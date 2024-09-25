"""Tab for displaying the output of a script.

This could be test results, logs, or other output from pre-request or
post-response scripts.
"""

from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical, VerticalScroll
from textual.widgets import Label


class ScriptOutput(VerticalScroll):
    DEFAULT_CSS = """\
        ScriptOutput {
            padding: 0 2;
        }    
    """

    def compose(self) -> ComposeResult:
        with Horizontal():
            with Vertical():
                yield Label("Pre-request")
                yield Label("Status")

            with Vertical():
                yield Label("Post-request")
                yield Label("Status")
