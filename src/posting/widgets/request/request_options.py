from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical, VerticalScroll
from textual.events import DescendantFocus
from textual.widgets import Checkbox, Input, Label, Static

from posting.collection import Options


class RequestOptions(VerticalScroll):
    DEFAULT_CSS = """\
    RequestOptions {
    
        Checkbox {
            height: 1;
            margin: 0 2 1 2;
            padding: 0 1;
            border: none;
            background: transparent;
            &:focus {
                border: none;
                background: $accent 20%;
                color: $text;
                padding: 0 1 0 0;
                border-left: wide $accent;
                & .toggle--label {
                    text-style: not underline;
                }
            }
        }

        #proxy-option {
            padding-left: 3;
            margin-bottom: 1;
            height: auto;
        }

        #timeout-option {
            padding-left: 3;
            height: auto;
            margin-bottom: 1;
        }

        & #option-description {
            dock: right;
            height: 1fr;
            width: 50%;
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

    def __init__(self):
        super().__init__()
        self.can_focus = False

        self.options = Options()

        self.descriptions = {
            "follow-redirects": "Follow redirects when the server responds with a 3xx status code.",
            "verify": "Verify SSL certificates when making requests.",
            "attach-cookies": "Attach cookies to outgoing requests to the same domain.",
            "proxy-url": "Proxy URL to use for requests.\ne.g. http://user:password@localhost:8080",
            "timeout": "Timeout for the request in seconds.",
        }

    def compose(self) -> ComposeResult:
        yield Checkbox(
            "Follow redirects",
            value=self.options.follow_redirects,
            id="follow-redirects",
        )
        yield Checkbox(
            "Verify SSL certificates",
            value=self.options.verify_ssl,
            id="verify",
        )
        yield Checkbox(
            "Attach cookies",
            value=self.options.attach_cookies,
            id="attach-cookies",
        )

        with Vertical(id="proxy-option"):
            yield Label("Proxy URL")
            yield Input(id="proxy-url")

        with Vertical(id="timeout-option"):
            yield Label("Timeout")
            yield Input(
                value=str(self.options.timeout),
                id="timeout",
                type="number",
                validate_on=["changed"],
            )

        # A panel which the description of the option will be
        # displayed inside.
        yield Static("", id="option-description")

    @on(Checkbox.Changed)
    def on_checkbox_change(self, event: Checkbox.Changed) -> None:
        """Handle the checkbox change event."""
        match event.checkbox.id:
            case "follow-redirects":
                self.options.follow_redirects = event.value
            case "verify":
                self.options.verify_ssl = event.value
            case "attach-cookies":
                self.options.attach_cookies = event.value
            case _:
                pass

    @on(Input.Changed, selector="#proxy-url")
    def on_proxy_url_changed(self, event: Input.Changed) -> None:
        """Handle the input change event."""
        self.options.proxy_url = event.value

    @on(Input.Changed, selector="#timeout")
    def on_timeout_changed(self, event: Input.Changed) -> None:
        """Handle the input change event."""
        try:
            self.options.timeout = float(event.value)
        except ValueError:
            self.options.timeout = 5.0

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
        return self.options

    def load_options(self, options: Options) -> None:
        """Load the options into the widget.

        Note that this is just setting the values on the widget.

        The change events emitted from the widgets are the things that
        result in the internal Options model state being updated.
        """
        self.follow_redirects_checkbox.value = options.follow_redirects
        self.verify_ssl_checkbox.value = options.verify_ssl
        self.attach_cookies_checkbox.value = options.attach_cookies
        self.proxy_url_input.value = options.proxy_url
        self.timeout_input.value = str(options.timeout)

    @property
    def follow_redirects_checkbox(self) -> Checkbox:
        return self.query_one("#follow-redirects", Checkbox)

    @property
    def verify_ssl_checkbox(self) -> Checkbox:
        return self.query_one("#verify", Checkbox)

    @property
    def attach_cookies_checkbox(self) -> Checkbox:
        return self.query_one("#attach-cookies", Checkbox)

    @property
    def proxy_url_input(self) -> Input:
        return self.query_one("#proxy-url", Input)

    @property
    def timeout_input(self) -> Input:
        return self.query_one("#timeout", Input)
