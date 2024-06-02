from rich.console import Console, ConsoleOptions, RenderResult as RichRenderResult
from rich.segment import Segment
from rich.style import Style
from textual.app import RenderResult
from textual.widgets import Static


class CrosshatchRenderable:
    def __init__(self, character: str = "╲", style: Style = Style.null()) -> None:
        self.character = character
        self.style = style

    def __rich_console__(
        self, console: Console, options: ConsoleOptions
    ) -> RichRenderResult:
        style = console.get_style(self.style)
        character = self.character
        width = options.max_width
        blank_line = [Segment(f"{character * width}\n", style)]
        yield from blank_line * options.max_height


class Crosshatch(Static):
    DEFAULT_CSS = "Crosshatch { color: $accent-lighten-2 18%; }"

    def render(self) -> RenderResult:
        return CrosshatchRenderable()
