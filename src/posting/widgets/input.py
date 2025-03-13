from rich.style import Style
from textual.theme import Theme
from textual.widgets import Input


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
