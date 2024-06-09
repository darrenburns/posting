from textual import on
from textual.app import ComposeResult
from textual.containers import Vertical
from textual.reactive import reactive
from textual.widgets import Checkbox, Label


class RequestOptions(Vertical):
    DEFAULT_CSS = """\
    #options-under-construction {
        color: $error;
        padding: 1 2;
    }
    """

    follow_redirects = reactive(False)
    verify = reactive(False)
    attach_cookies = reactive(True)

    def __init__(self):
        super().__init__()
        # TODO - set the default values from config here.

    def compose(self) -> ComposeResult:
        yield Label(
            "This section is under construction!", id="options-under-construction"
        )
        yield Checkbox(
            "Follow redirects",
            value=self.follow_redirects,
            id="follow-redirects",
        )
        yield Checkbox(
            "Verify SSL/TLS certificates",
            value=self.verify,
            id="verify",
        )
        yield Checkbox(
            "Attach cookies",
            value=self.attach_cookies,
            id="attach-cookies",
        )

    @on(Checkbox.Changed)
    def on_checkbox_change(self, event: Checkbox.Changed) -> None:
        match event.checkbox.id:
            case "follow-redirects":
                self.follow_redirects = event.value
            case "verify":
                self.verify = event.value
            case "attach-cookies":
                self.attach_cookies = event.value
            case _:
                pass
