from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import VerticalScroll
from textual.events import DescendantFocus
from textual.reactive import reactive
from textual.widgets import Checkbox, Static


class RequestOptions(VerticalScroll):
    DEFAULT_CSS = """\
    RequestOptions {

        & #option-description {
            dock: right;
            height: 1fr;
            width: auto;
            max-width: 50%;
            background: transparent;
            color: $text-muted;
            padding: 1 2;
            display: none;
            border-left: $accent 30%;

            &.show {
                display: block;
            }
        }
    }
    """

    BINDINGS = [
        Binding("down,j", "screen.focus_next", "Next"),
        Binding("up,k", "screen.focus_previous", "Previous"),
    ]

    follow_redirects = reactive(False)
    verify = reactive(True)
    attach_cookies = reactive(True)

    def __init__(self):
        super().__init__()
        self.can_focus = False
        # TODO - set the default values from config here.

        self.descriptions = {
            "follow-redirects": "Follow redirects when the server responds with a 3xx status code.",
            "verify": "Verify SSL certificates when making requests.",
            "attach-cookies": "Attach cookies to requests to the same domain.",
        }

    def compose(self) -> ComposeResult:
        yield Checkbox(
            "Follow redirects",
            value=self.follow_redirects,
            id="follow-redirects",
        )
        yield Checkbox(
            "Verify SSL certificates",
            value=self.verify,
            id="verify",
        )
        yield Checkbox(
            "Attach cookies",
            value=self.attach_cookies,
            id="attach-cookies",
        )
        yield Static("", id="option-description")

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

    @on(DescendantFocus)
    def on_descendant_focus(self, event: DescendantFocus) -> None:
        focused_id = event.control.id
        description_label = self.query_one("#option-description", Static)
        has_description = focused_id in self.descriptions
        description_label.set_class(has_description, "show")
        if has_description:
            description_label.update(self.descriptions[focused_id])
        else:
            description_label.update("")
