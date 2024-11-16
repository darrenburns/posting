from dataclasses import dataclass
from rich.console import RenderableType

from textual import on
from textual.binding import Binding
from textual.message import Message
from textual.widgets import Select
from posting.collection import HttpRequestMethod
from posting.help_screen import HelpData

from posting.widgets.select import PostingSelect


class MethodSelector(PostingSelect[str]):
    help = HelpData(
        title="Method Selector",
        description="""\
Select the HTTP method for the request.
You can select a method by typing a single letter. For example, pressing `g`
will set the method to `GET`.
The dropdown does not need to be expanded in order to select a method.
""",
    )

    BINDING_GROUP_TITLE = "HTTP Method Selector"

    BINDINGS = [
        Binding("g", "select_method('GET')", "GET", show=False),
        Binding("p", "select_method('POST')", "POST", show=False),
        Binding("a", "select_method('PATCH')", "PATCH", show=False),
        Binding("u", "select_method('PUT')", "PUT", show=False),
        Binding("d", "select_method('DELETE')", "DELETE", show=False),
        Binding("o", "select_method('OPTIONS')", "OPTIONS", show=False),
        Binding("h", "select_method('HEAD')", "HEAD", show=False),
    ]

    def __init__(
        self,
        *,
        prompt: str = "Method",
        value: str = "GET",
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
        tooltip: RenderableType | None = None,
    ):
        super().__init__(
            [
                ("[u]G[/]ET", "GET"),
                ("[u]P[/]OST", "POST"),
                ("P[u]U[/]T", "PUT"),
                ("[u]D[/]ELETE", "DELETE"),
                ("P[u]A[/]TCH", "PATCH"),
                ("[u]H[/]EAD", "HEAD"),
                ("[u]O[/]PTIONS", "OPTIONS"),
            ],
            prompt=prompt,
            allow_blank=False,
            value=value,
            name=name,
            id=id,
            classes=classes,
            disabled=disabled,
            tooltip=tooltip,
        )

    @dataclass
    class MethodChanged(Message):
        value: HttpRequestMethod
        select: "MethodSelector"

        @property
        def control(self) -> "MethodSelector":
            return self.select

    @on(Select.Changed)
    def method_selected(self, event: Select.Changed) -> None:
        event.stop()
        if event.value is not Select.BLANK:
            self.post_message(
                MethodSelector.MethodChanged(value=event.value, select=self)
            )

    def action_select_method(self, method: str) -> None:
        self.value = method
