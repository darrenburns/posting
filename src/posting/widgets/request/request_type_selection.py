from dataclasses import dataclass
from rich.console import RenderableType

from textual import on
from textual.binding import Binding
from textual.message import Message
from textual.widgets import Select
from posting.collection import RequestType
from posting.help_screen import HelpData

from posting.widgets.select import PostingSelect


class RequestTypeSelector(PostingSelect[str]):
    help = HelpData(
        title="Request Type Selector",
        description="""\
Select the type of the request.
You can select a type by typing a single letter. For example, pressing `g`
will set the type to `GET`.
The dropdown does not need to be expanded in order to select a type.
""",
    )

    BINDING_GROUP_TITLE = "Request Type Selector"

    BINDINGS = [
        Binding("g", "select_type('GET')", "GET", show=False),
        Binding("p", "select_type('POST')", "POST", show=False),
        Binding("a", "select_type('PATCH')", "PATCH", show=False),
        Binding("u", "select_type('PUT')", "PUT", show=False),
        Binding("d", "select_type('DELETE')", "DELETE", show=False),
        Binding("o", "select_type('OPTIONS')", "OPTIONS", show=False),
        Binding("h", "select_type('HEAD')", "HEAD", show=False),
        Binding("w", "select_type('WEBSOCKET')", "WEBSOCKET", show=False),
    ]

    def __init__(
        self,
        *,
        prompt: str = "Type",
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
                ("[u]W[/]EBSOCKET", "WEBSOCKET"),
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
    class TypeChanged(Message):
        value: RequestType
        select: "RequestTypeSelector"

        @property
        def control(self) -> "RequestTypeSelector":
            return self.select

    @on(Select.Changed)
    def method_selected(self, event: Select.Changed) -> None:
        event.stop()
        if event.value is not Select.BLANK:
            self.post_message(
                RequestTypeSelector.TypeChanged(value=event.value, select=self)
            )

    def action_select_type(self, type: RequestType) -> None:
        self.value = type
