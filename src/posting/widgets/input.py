from rich.console import Console, ConsoleOptions, RenderResult as RichRenderResult
from rich.segment import Segment
from rich.style import Style
from rich.text import Text
from textual.app import RenderResult
from textual.theme import Theme
from textual.widgets import Input
from textual._segment_tools import line_crop


from posting.config import SETTINGS


class PostingInput(Input):
    def on_mount(self) -> None:
        self.cursor_blink = SETTINGS.get().text_input.blinking_cursor

        self._theme_cursor_style: Style | None = None

        self.on_theme_change(self.app.current_theme)
        self.app.theme_changed_signal.subscribe(self, self.on_theme_change)

    @property
    def cursor_style(self) -> Style:
        return (
            self._theme_cursor_style
            if self._theme_cursor_style is not None
            else self.get_component_rich_style("input--cursor")
        )

    def on_theme_change(self, theme: Theme) -> None:
        cursor_style = theme.variables.get("input-cursor")
        self._theme_cursor_style = Style.parse(cursor_style) if cursor_style else None
        self.refresh()


class _InputRenderable:
    """Render the input content."""

    def __init__(self, input: PostingInput, cursor_visible: bool) -> None:
        self.input = input
        self.cursor_visible = cursor_visible

    def __rich_console__(
        self, console: "Console", options: "ConsoleOptions"
    ) -> RichRenderResult:
        input = self.input
        result = input._value
        width = input.content_size.width

        # Add the completion with a faded style.
        value = input.value
        value_length = len(value)
        suggestion = input._suggestion
        show_suggestion = len(suggestion) > value_length and input.has_focus
        if show_suggestion:
            result += Text(
                suggestion[value_length:],
                input.get_component_rich_style("input--suggestion"),
            )

        if self.cursor_visible and input.has_focus:
            if not show_suggestion and input._cursor_at_end:
                result.pad_right(1)
            cursor_position = input.cursor_position
            cursor_style = input.cursor_style
            result.stylize(cursor_style, cursor_position, cursor_position + 1)

        segments = list(result.render(console))
        line_length = Segment.get_line_length(segments)
        if line_length < width:
            segments = Segment.adjust_line_length(segments, width)
            line_length = width

        line = line_crop(
            list(segments),
            input.view_position,
            input.view_position + width,
            line_length,
        )
        yield from line
