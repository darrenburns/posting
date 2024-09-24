"""Tab for displaying the output of a script.

This could be test results, logs, or other output from pre-request or
post-response scripts.
"""

from textual.app import ComposeResult
from textual.containers import VerticalScroll
from textual.widgets import RichLog, TabPane, TabbedContent


class ScriptOutput(VerticalScroll):
    DEFAULT_CSS = """\
        ScriptOutput {
            padding: 0 2;
        }    
    """

    def compose(self) -> ComposeResult:
        # Tabs for test results, logs, etc.
        with TabbedContent(id="script-output-tabs"):
            with TabPane("Output"):
                yield RichLog()
            with TabPane("Logs"):
                yield RichLog()
            # TODO: Add tests tab
            # with TabPane("Tests"):
            # yield Label("Test results will appear here.")
