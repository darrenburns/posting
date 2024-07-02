from typing import Protocol, runtime_checkable
import httpx
from textual import on, log
from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical, VerticalScroll
from textual.widgets import ContentSwitcher, Input, Label, Select, Static

from posting.collection import Auth


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
        yield Input(
            placeholder="Enter a username",
            id="username-input",
        )
        yield Label("Password")
        yield Input(placeholder="Enter a password", password=True, id="password-input")

    def set_values(self, username: str, password: str) -> None:
        self.query_one("#username-input", Input).value = username
        self.query_one("#password-input", Input).value = password

    def get_values(self) -> dict[str, str]:
        return {
            "username": self.query_one("#username-input", Input).value,
            "password": self.query_one("#password-input", Input).value,
        }


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

    def compose(self) -> ComposeResult:
        self.can_focus = False
        with Horizontal():
            with Vertical():
                yield Label("Auth type ", id="auth-type-label")
                yield Select(
                    options=[
                        ("No Auth", None),
                        ("Basic", "basic"),
                        ("Digest", "digest"),
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
                    auth.basic.username.get_secret_value(),
                    auth.basic.password.get_secret_value(),
                )
            case "digest":
                if auth.digest is None:
                    log.warning(
                        "Digest auth selected, but no values provided for username or password."
                    )
                    return
                self.query_one("#auth-type-select", Select).value = "digest"
                self.query_one("#auth-form-digest", UserNamePasswordForm).set_values(
                    auth.digest.username.get_secret_value(),
                    auth.digest.password.get_secret_value(),
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
