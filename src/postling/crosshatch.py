from rich.console import Console, ConsoleOptions, RenderResult
from rich.segment import Segment
from rich.style import Style


class Crosshatch:
    def __init__(self, character: str = "╲", style: Style = Style.null()) -> None:
        self.character = character
        self.style = style

    def __rich_console__(
        self, console: Console, options: ConsoleOptions
    ) -> RenderResult:
        style = console.get_style(self.style)
        character = self.character
        width = options.max_width
        blank_line = [Segment(f"{character * width}\n", style)]
        yield from blank_line * options.max_height
