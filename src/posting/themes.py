import uuid
from pydantic import BaseModel, Field
from rich.style import Style
from textual.design import ColorSystem
from textual.widgets.text_area import TextAreaTheme
import yaml
from posting.config import SETTINGS


class PostingTextAreaTheme(BaseModel):
    gutter: str | None = Field(default=None)
    """The style to apply to the gutter."""

    cursor: str | None = Field(default=None)
    """The style to apply to the cursor."""

    cursor_line: str | None = Field(default=None)
    """The style to apply to the line the cursor is on."""

    cursor_line_gutter: str | None = Field(default=None)
    """The style to apply to the gutter of the line the cursor is on."""

    matched_bracket: str | None = Field(default=None)
    """The style to apply to bracket matching."""

    selection: str | None = Field(default=None)
    """The style to apply to the selected text."""


class SyntaxTheme(BaseModel):
    """Colours used in highlighting syntax in text areas and
    URL input fields."""

    json_key: str | None = Field(default=None)
    """The style to apply to JSON keys."""

    json_string: str | None = Field(default=None)
    """The style to apply to JSON strings."""

    json_number: str | None = Field(default=None)
    """The style to apply to JSON numbers."""

    json_boolean: str | None = Field(default=None)
    """The style to apply to JSON booleans."""

    json_null: str | None = Field(default=None)
    """The style to apply to JSON null values."""

    def to_text_area_syntax_styles(self, fallback_theme: "Theme") -> dict[str, Style]:
        """Convert this theme to a TextAreaTheme.

        If a fallback theme is provided, it will be used to fill in any missing
        styles.
        """
        syntax_styles = {
            "string": Style.parse(self.json_string or fallback_theme.primary),
            "number": Style.parse(self.json_number or fallback_theme.accent),
            "boolean": Style.parse(self.json_boolean or fallback_theme.accent),
            "json.null": Style.parse(self.json_null or fallback_theme.secondary),
            "json.label": (
                Style.parse(self.json_key or fallback_theme.primary) + Style(bold=True)
            ),
        }
        return syntax_styles


class VariableStyles(BaseModel):
    """The style to apply to variables."""

    resolved: str | None = Field(default=None)
    """The style to apply to resolved variables."""

    unresolved: str | None = Field(default=None)
    """The style to apply to unresolved variables."""

    def fill_with_defaults(self, theme: "Theme") -> "VariableStyles":
        """Return a new VariableStyles object with `None` values filled
        with reasonable defaults from the given theme."""
        return VariableStyles(
            resolved=self.resolved or theme.success,
            unresolved=self.unresolved or theme.error,
        )


class UrlStyles(BaseModel):
    """The style to apply to URL input fields."""

    base: str | None = Field(default=None)
    """The style to apply to the base of the URL."""

    protocol: str | None = Field(default=None)
    """The style to apply to the URL protocol."""

    separator: str | None = Field(default="dim")
    """The style to apply to URL separators e.g. `/`."""

    def fill_with_defaults(self, theme: "Theme") -> "UrlStyles":
        """Return a new UrlStyles object with `None` values filled
        with reasonable defaults from the given theme."""
        return UrlStyles(
            base=self.base or theme.secondary,
            protocol=self.protocol or theme.accent,
            separator=self.separator or "dim",
        )


class Theme(BaseModel):
    name: str = Field(exclude=True)
    primary: str
    secondary: str | None = None
    background: str | None = None
    surface: str | None = None
    panel: str | None = None
    warning: str | None = None
    error: str | None = None
    success: str | None = None
    accent: str | None = None
    dark: bool = True

    text_area: PostingTextAreaTheme = Field(default_factory=PostingTextAreaTheme)
    """Styling to apply to TextAreas."""

    syntax: str | SyntaxTheme = Field(default="posting", exclude=True)
    """Posting can associate a syntax highlighting theme which will
    be switched to automatically when the app theme changes.
    
    This can either be a custom SyntaxTheme or a pre-defined Textual theme
    such as monokai, dracula, github_light, or vscode_dark."""

    url: UrlStyles | None = Field(default_factory=UrlStyles)
    """Styling to apply to URL input fields."""

    variable: VariableStyles | None = Field(default_factory=VariableStyles)
    """The style to apply to variables."""

    # Optional metadata
    author: str | None = Field(default=None, exclude=True)
    description: str | None = Field(default=None, exclude=True)
    homepage: str | None = Field(default=None, exclude=True)

    def to_color_system(self) -> ColorSystem:
        """Convert this theme to a ColorSystem."""
        return ColorSystem(
            **self.model_dump(
                exclude={
                    "text_area",
                    "syntax",
                    "variable",
                    "url",
                }
            )
        )

    def to_text_area_theme(self) -> TextAreaTheme:
        """Retrieve the TextAreaTheme corresponding to this theme."""
        syntax_styles: dict[str, Style] = {}
        if isinstance(self.syntax, SyntaxTheme):
            syntax_styles = self.syntax.to_text_area_syntax_styles(self)

        text_area = self.text_area
        return TextAreaTheme(
            name=uuid.uuid4().hex,
            syntax_styles=syntax_styles,
            gutter_style=Style.parse(text_area.gutter) if text_area.gutter else None,
            cursor_style=Style.parse(text_area.cursor) if text_area.cursor else None,
            cursor_line_style=Style.parse(text_area.cursor_line)
            if text_area.cursor_line
            else None,
            cursor_line_gutter_style=Style.parse(text_area.cursor_line_gutter)
            if text_area.cursor_line_gutter
            else None,
            bracket_matching_style=Style.parse(text_area.matched_bracket)
            if text_area.matched_bracket
            else None,
            selection_style=Style.parse(text_area.selection)
            if text_area.selection
            else None,
        )


def load_user_themes() -> dict[str, Theme]:
    """Load user themes from "~/.config/posting/themes".

    Returns:
        A dictionary mapping theme names to theme objects.
    """
    directory = SETTINGS.get().theme_directory
    themes: dict[str, Theme] = {}
    for path in directory.iterdir():
        path_suffix = path.suffix
        if path_suffix == ".yaml" or path_suffix == ".yml":
            with path.open() as theme_file:
                theme_content = yaml.load(theme_file, Loader=yaml.FullLoader) or {}
                try:
                    themes[theme_content["name"]] = Theme(**theme_content)
                except KeyError:
                    raise ValueError(
                        f"Invalid theme file {path}. A `name` is required."
                    )
    return themes


BUILTIN_THEMES: dict[str, Theme] = {
    "posting": Theme(
        name="posting",
        primary="#004578",
        secondary="#0178D4",
        warning="#ffa62b",
        error="#ba3c5b",
        success="#4EBF71",
        accent="#ffa62b",
        dark=True,
        syntax="posting",
    ),
    "monokai": Theme(
        name="monokai",
        primary="#F92672",  # Pink
        secondary="#66D9EF",  # Light Blue
        warning="#FD971F",  # Orange
        error="#F92672",  # Pink (same as primary for consistency)
        success="#A6E22E",  # Green
        accent="#AE81FF",  # Purple
        background="#272822",  # Dark gray-green
        surface="#3E3D32",  # Slightly lighter gray-green
        panel="#3E3D32",  # Same as surface for consistency
        dark=True,
        syntax="monokai",
    ),
    "solarized-light": Theme(
        name="solarized-light",
        primary="#268bd2",
        secondary="#2aa198",
        warning="#cb4b16",
        error="#dc322f",
        success="#859900",
        accent="#6c71c4",
        background="#fdf6e3",
        surface="#eee8d5",
        panel="#eee8d5",
        syntax="github_light",
    ),
    "nautilus": Theme(
        name="nautilus",
        primary="#0077BE",  # Ocean Blue
        secondary="#20B2AA",  # Light Sea Green
        warning="#FFD700",  # Gold (like sunlight on water)
        error="#FF6347",  # Tomato (like a warning buoy)
        success="#32CD32",  # Lime Green (like seaweed)
        accent="#FF8C00",  # Dark Orange (like a sunset over water)
        dark=True,
        background="#001F3F",  # Dark Blue (deep ocean)
        surface="#003366",  # Navy Blue (shallower water)
        panel="#005A8C",  # Steel Blue (water surface)
        syntax="posting",
    ),
    "galaxy": Theme(
        name="galaxy",
        primary="#8A2BE2",  # Improved Deep Magenta (Blueviolet)
        secondary="#9370DB",  # Softer Dusky Indigo (Medium Purple)
        warning="#FFD700",  # Gold, more visible than orange
        error="#FF4500",  # OrangeRed, vibrant but less harsh than pure red
        success="#00FA9A",  # Medium Spring Green, kept for vibrancy
        accent="#FF69B4",  # Hot Pink, for a pop of color
        dark=True,
        background="#0F0F1F",  # Very Dark Blue, almost black
        surface="#1E1E3F",  # Dark Blue-Purple
        panel="#2D2B55",  # Slightly Lighter Blue-Purple
        syntax="dracula",
    ),
    "nebula": Theme(
        name="nebula",
        primary="#4169E1",  # Royal Blue, more vibrant than Midnight Blue
        secondary="#9400D3",  # Dark Violet, more vibrant than Indigo Dye
        warning="#FFD700",  # Kept Gold for warnings
        error="#FF1493",  # Deep Pink, more nebula-like than Crimson
        success="#00FF7F",  # Spring Green, slightly more vibrant
        accent="#FF00FF",  # Magenta, for a true neon accent
        dark=True,
        background="#0A0A23",  # Dark Navy, closer to a night sky
        surface="#1C1C3C",  # Dark Blue-Purple
        panel="#2E2E5E",  # Slightly Lighter Blue-Purple
        syntax="dracula",
    ),
    "alpine": Theme(
        name="alpine",
        primary="#4A90E2",  # Clear Sky Blue
        secondary="#81A1C1",  # Misty Blue
        warning="#EBCB8B",  # Soft Sunlight
        error="#BF616A",  # Muted Red
        success="#A3BE8C",  # Alpine Meadow Green
        accent="#5E81AC",  # Mountain Lake Blue
        dark=True,
        background="#2E3440",  # Dark Slate Grey
        surface="#3B4252",  # Darker Blue-Grey
        panel="#434C5E",  # Lighter Blue-Grey
    ),
    "cobalt": Theme(
        name="cobalt",
        primary="#334D5C",  # Deep Cobalt Blue
        secondary="#4878A6",  # Slate Blue
        warning="#FFAA22",  # Amber, suitable for warnings related to primary
        error="#E63946",  # Red, universally recognized for errors
        success="#4CAF50",  # Green, commonly used for success indication
        accent="#D94E64",  # Candy Apple Red
        dark=True,
        surface="#27343B",  # Dark Lead
        panel="#2D3E46",  # Storm Gray
        background="#1F262A",  # Charcoal
    ),
    "twilight": Theme(
        name="twilight",
        primary="#367588",
        secondary="#5F9EA0",
        warning="#FFD700",
        error="#FF6347",
        success="#00FA9A",
        accent="#FF7F50",
        dark=True,
        background="#191970",
        surface="#3B3B6D",
        panel="#4C516D",
    ),
    "hacker": Theme(
        name="hacker",
        primary="#00FF00",  # Bright Green (Lime)
        secondary="#32CD32",  # Lime Green
        warning="#ADFF2F",  # Green Yellow
        error="#FF4500",  # Orange Red (for contrast)
        success="#00FA9A",  # Medium Spring Green
        accent="#39FF14",  # Neon Green
        dark=True,
        background="#0D0D0D",  # Almost Black
        surface="#1A1A1A",  # Very Dark Gray
        panel="#2A2A2A",  # Dark Gray
    ),
}
