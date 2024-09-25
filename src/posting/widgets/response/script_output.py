"""Tab for displaying the output of a script.

This could be test results, logs, or other output from pre-request or
post-response scripts.
"""

from typing import Literal
from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical, VerticalScroll
from textual.reactive import Reactive, reactive
from textual.widgets import Label

ScriptStatus = Literal["success", "error", "no-script"]


class ScriptOutput(VerticalScroll):
    DEFAULT_CSS = """\
        ScriptOutput {
            padding: 0 2;

            & Label {
                &.-success {
                    color: $success;
                }
                &.-error {
                    color: $error;
                }
                &.-no-script {
                    color: $text-muted;
                }
            }
        }    
    """

    request_status: Reactive[ScriptStatus] = reactive("no-script")
    response_status: Reactive[ScriptStatus] = reactive("no-script")

    def compose(self) -> ComposeResult:
        with Horizontal():
            with Vertical():
                yield Label("Pre-request")
                yield Label(self.request_status, id="request-status")

            with Vertical():
                yield Label("Post-request")
                yield Label(self.response_status, id="response-status")

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
                label.update("Success")
            elif error:
                label.update("Error")
            elif no_script:
                label.update("No script")
