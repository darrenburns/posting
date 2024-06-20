from typing import Protocol, runtime_checkable
import httpx
from textual import on
from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical, VerticalScroll
from textual.widgets import ContentSwitcher, Input, Label, Select, Static


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
        content_switcher = self.query_one("#auth-form-switcher", ContentSwitcher)
        value = event.value
        content_switcher.current = f"auth-form-{value}" if value else None

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
