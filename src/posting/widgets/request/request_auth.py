from typing import Protocol, runtime_checkable
import httpx
from textual import on, log
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical, VerticalScroll
from textual.types import InputValidationOn
from textual.validation import Length
from textual.widgets import ContentSwitcher, Input, Label, Select, Static

from posting.auth import HttpxBearerTokenAuth
from posting.collection import Auth, BasicAuth, BearerTokenAuth, DigestAuth
from posting.widgets.select import PostingSelect
from posting.widgets.variable_input import VariableInput


@runtime_checkable
class Form(Protocol):
    id: str | None
    """The ID of the form."""

    def get_values(self) -> dict[str, str]:
        """Get the values from the form as a dict."""
        ...


class UserNamePasswordForm(Vertical):
    DEFAULT_CSS = """
    UserNamePasswordForm {
        padding: 1 0;

        & #username-input {
            margin-bottom: 1;
        }
    }
    """

    def compose(self) -> ComposeResult:
        yield Label("Username")
        yield VariableInput(
            placeholder="Enter a username",
            id="username-input",
        )
        yield Label("Password")
        yield VariableInput(placeholder="Enter a password", id="password-input")

    def set_values(self, username: str, password: str) -> None:
        self.query_one("#username-input", Input).value = username
        self.query_one("#password-input", Input).value = password

    def get_values(self) -> dict[str, str]:
        return {
            "username": self.query_one("#username-input", Input).value,
            "password": self.query_one("#password-input", Input).value,
        }


class BearerTokenForm(Vertical):
    DEFAULT_CSS = """
    BearerTokenForm {
        padding: 1 0;

        & #token-input {
            margin-bottom: 1;
        }

        & #token-empty-label {
            dock: right;
            padding: 0 1;
            color: $text-error;
        }
    }
    """

    def compose(self) -> ComposeResult:
        with Horizontal():
            yield Label("Token")
            yield Label(
                "Should not be empty",
                id="token-empty-label",
            )
        yield VariableInput(
            placeholder="Enter a token",
            password=True,
            validators=[Length(minimum=1)],
            validate_on=["changed"],
            id="token-input",
        )

    def on_mount(self) -> None:
        token_input = self.token_input
        token_input.validate(token_input.value)

    @on(Input.Changed)
    def on_input_changed(self, event: Input.Changed) -> None:
        result = event.validation_result
        is_valid = False if result is None else result.is_valid
        label = self.query_one("#token-empty-label", Label)
        label.visible = not is_valid

    def set_values(self, token: str) -> None:
        self.token_input.value = token

    def get_values(self) -> dict[str, str]:
        return {
            "token": self.token_input.value,
        }

    @property
    def token_input(self) -> Input:
        return self.query_one("#token-input", Input)


class RequestAuth(VerticalScroll):
    DEFAULT_CSS = """
    RequestAuth {
        padding: 0 2 1 2;

        & Horizontal, & Vertical {
            height: auto;
        }

        & #auth-type-description {
            color: $text-muted;
            width: 1fr;
            padding: 0 1;
        }

        & Select#auth-type-select {
            width: 1fr;
        }

        & #auth-form-switcher {
            padding: 0;
        }

    }
    """

    BINDINGS = [
        Binding("up", "screen.focus_previous", "Focus previous", show=False),
        Binding("down", "screen.focus_next", "Focus next", show=False),
    ]

    def compose(self) -> ComposeResult:
        self.can_focus = False
        with Horizontal():
            with Vertical():
                yield Label("Auth type ", id="auth-type-label")
                yield PostingSelect(
                    options=[
                        ("No Auth", None),
                        ("Basic", "basic"),
                        ("Digest", "digest"),
                        ("Bearer Token", "bearer-token"),
                    ],
                    allow_blank=False,
                    prompt="Auth Type",
                    value=None,
                    id="auth-type-select",
                )
            yield Static(
                "Authorization headers will be generated when the request is sent.",
                id="auth-type-description",
            )

        with ContentSwitcher(initial=None, id="auth-form-switcher"):
            yield UserNamePasswordForm(id="auth-form-basic")
            yield UserNamePasswordForm(id="auth-form-digest")
            yield BearerTokenForm(id="auth-form-bearer-token")

    @on(Select.Changed, selector="#auth-type-select")
    def on_auth_type_changed(self, event: Select.Changed):
        value = event.value
        self.content_switcher.current = f"auth-form-{value}" if value else None

    def to_httpx_auth(self) -> httpx.Auth | None:
        form = self.current_form
        if form is None:
            return None

        match form.id:
            case "auth-form-basic":
                return httpx.BasicAuth(**form.get_values())
            case "auth-form-digest":
                return httpx.DigestAuth(**form.get_values())
            case "auth-form-bearer-token":
                return HttpxBearerTokenAuth(**form.get_values())
            case _:
                return None

    def to_model(self) -> Auth | None:
        form = self.current_form
        if form is None:
            return None

        match form.id:
            case "auth-form-basic":
                form_values = form.get_values()
                username = form_values["username"]
                password = form_values["password"]
                return Auth(
                    type="basic", basic=BasicAuth(username=username, password=password)
                )
            case "auth-form-digest":
                form_values = form.get_values()
                username = form_values["username"]
                password = form_values["password"]
                return Auth(
                    type="digest",
                    digest=DigestAuth(username=username, password=password),
                )
            case "auth-form-bearer-token":
                form_values = form.get_values()
                token = form_values["token"]
                return Auth(
                    type="bearer_token", bearer_token=BearerTokenAuth(token=token)
                )
            case _:
                return None

    def load_auth(self, auth: Auth | None) -> None:
        if auth is None:
            self.query_one("#auth-type-select", Select).value = None
            return
        match auth.type:
            case "basic":
                self.query_one("#auth-type-select", Select).value = "basic"
                if auth.basic is None:
                    log.warning(
                        "Basic auth selected, but no values provided for username or password."
                    )
                    return
                self.query_one("#auth-form-basic", UserNamePasswordForm).set_values(
                    auth.basic.username,
                    auth.basic.password,
                )
            case "digest":
                if auth.digest is None:
                    log.warning(
                        "Digest auth selected, but no values provided for username or password."
                    )
                    return
                self.query_one("#auth-type-select", Select).value = "digest"
                self.query_one("#auth-form-digest", UserNamePasswordForm).set_values(
                    auth.digest.username,
                    auth.digest.password,
                )
            case "bearer_token":
                if auth.bearer_token is None:
                    log.warning(
                        "Bearer auth selected, but no values provided for token."
                    )
                    return
                self.query_one("#auth-type-select", Select).value = "bearer-token"
                self.query_one("#auth-form-bearer-token", BearerTokenForm).set_values(
                    auth.bearer_token.token
                )
            case _:
                log.warning(f"Unknown auth type: {auth.type}")

    @property
    def content_switcher(self) -> ContentSwitcher:
        return self.query_one("#auth-form-switcher", ContentSwitcher)

    @property
    def current_form(self) -> Form | None:
        current_id = self.content_switcher.current
        if current_id is None:
            return None
        form = self.query_one(f"#{current_id}")
        if not isinstance(form, Form):
            return None
        return form
