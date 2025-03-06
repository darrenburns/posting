"""Tab for displaying the output of a script.
https://github.com/sergeyklay/gohugo-theme-ed/blob/main/exampleSite/hugo.toml
This could be test results, logs, or other output from pre-request or
post-response scripts.
"""

from typing import Literal
from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical, VerticalScroll
from textual.reactive import Reactive, reactive
from textual.widgets import Label, RichLog

from posting.help_screen import HelpData

ScriptStatus = Literal["success", "error", "no-script"]


class ScriptOutput(VerticalScroll):
    help = HelpData(
        title="Script Output",
        description="""\
This log displays the output of scripts that executed during the last request.
""",
    )

    BINDING_GROUP_TITLE = "Script Output"

    setup_status: Reactive[ScriptStatus] = reactive("no-script")
    request_status: Reactive[ScriptStatus] = reactive("no-script")
    response_status: Reactive[ScriptStatus] = reactive("no-script")

    def compose(self) -> ComposeResult:
        self.can_focus = False
        with Horizontal(id="status-bar"):
            with Vertical():
                yield Label("Setup")
                yield Label(self.setup_status, id="setup-status")
            with Vertical():
                yield Label("Pre-request")
                yield Label(self.request_status, id="request-status")
            with Vertical():
                yield Label("Post-response")
                yield Label(self.response_status, id="response-status")

        yield Label("Script output", id="script-output-title")
        yield RichLog(markup=True, highlight=True)

    def set_setup_status(self, status: ScriptStatus) -> None:
        """Set the status of the setup script."""
        self.setup_status = status
        self.set_label_status("setup-status", status)

    def set_request_status(self, status: ScriptStatus) -> None:
        """Set the status of the request."""
        self.request_status = status
        self.set_label_status("request-status", status)

    def set_response_status(self, status: ScriptStatus) -> None:
        """Set the status of the response."""
        self.response_status = status
        self.set_label_status("response-status", status)

    def set_label_status(self, label_id: str, status: ScriptStatus) -> None:
        """Set the status of a label."""
        label = self.query_one(f"#{label_id}")
        success = status == "success"
        error = status == "error"
        no_script = status == "no-script"

        if isinstance(label, Label):
            label.set_class(success, "-success")
            label.set_class(error, "-error")
            label.set_class(no_script, "-no-script")

            if success:
                label.update("Success ✔︎")
            elif error:
                label.update("Error ⨯")
            elif no_script:
                label.update("-")

    def reset(self) -> None:
        """Reset the output."""
        self.rich_log.clear()
        self.set_setup_status("no-script")
        self.set_request_status("no-script")
        self.set_response_status("no-script")

    def log_function_call_start(self, function: str) -> None:
        """Log the start of a function call."""
        self.rich_log.write(f"[b dim]Running {function}[/]")

    @property
    def rich_log(self) -> RichLog:
        """Get the RichLog widget which stdout and stderr are printed to."""
        return self.query_one(RichLog)
