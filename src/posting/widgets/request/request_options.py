from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import VerticalScroll
from textual.events import DescendantFocus
from textual.reactive import Reactive, reactive
from textual.widgets import Checkbox, Static

from posting.collection import Options


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

    follow_redirects: Reactive[bool] = reactive(True)
    verify_ssl: Reactive[bool] = reactive(True)
    attach_cookies: Reactive[bool] = reactive(True)

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
        ).data_bind(value=RequestOptions.follow_redirects)
        yield Checkbox(
            "Verify SSL certificates",
            value=self.verify_ssl,
            id="verify",
        ).data_bind(value=RequestOptions.verify_ssl)
        yield Checkbox(
            "Attach cookies",
            value=self.attach_cookies,
            id="attach-cookies",
        ).data_bind(value=RequestOptions.attach_cookies)

        # A panel which the description of the option will be
        # displayed inside.
        yield Static("", id="option-description")

    @on(Checkbox.Changed)
    def on_checkbox_change(self, event: Checkbox.Changed) -> None:
        """Handle the checkbox change event."""
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
        """Show the description of the option when the user focuses on it."""
        focused_id = event.control.id
        description_label = self.query_one("#option-description", Static)
        has_description = focused_id in self.descriptions
        description_label.set_class(has_description, "show")
        if has_description:
            description_label.update(self.descriptions[focused_id])
        else:
            description_label.update("")

    def to_model(self) -> Options:
        """Export the options to a model."""
        return Options(
            follow_redirects=self.follow_redirects,
            verify_ssl=self.verify,
            attach_cookies=self.attach_cookies,
        )

    def load_options(self, options: Options) -> None:
        """Load the options into the widget."""
        self.follow_redirects = options.follow_redirects
        self.verify = options.verify_ssl
        self.attach_cookies = options.attach_cookies
