import re
from rich.highlighter import Highlighter
from rich.style import Style
from rich.text import Text
from textual.widgets import Input
from posting.themes import UrlStyles, VariableStyles

from posting.variables import (
    find_variable_end,
    find_variable_start,
    find_variables,
    get_variables,
    is_cursor_within_variable,
)


_URL_REGEX = re.compile(r"(?P<protocol>https?)://(?P<base>[^/]+)(?P<path>/[^ ]*)?")


def highlight_url(text: Text, styles: UrlStyles) -> None:
    for match in _URL_REGEX.finditer(text.plain):
        protocol_start, protocol_end = match.span("protocol")
        base_start, base_end = match.span("base")
        separator_start, separator_end = protocol_end, protocol_end + 3

        text.stylize(styles.protocol or "#818cf8", protocol_start, protocol_end)
        text.stylize(styles.separator or "dim b", separator_start, separator_end)
        text.stylize(styles.base or "#00C168", base_start, base_end)

    for index, char in enumerate(text.plain):
        if char == "/":
            text.stylize(styles.separator or "dim b", index, index + 1)


def highlight_variables(text: Text, styles: VariableStyles) -> None:
    for match in find_variables(text.plain):
        variable_name, start, end = match
        if variable_name not in get_variables():
            text.stylize(Style.parse(styles.unresolved or "dim"), start, end)
        else:
            text.stylize(Style.parse(styles.resolved or ""), start, end)


class VariableHighlighter(Highlighter):
    def __init__(self, variable_styles: VariableStyles | None = None) -> None:
        super().__init__()
        self.variable_styles = variable_styles

    def highlight(self, text: Text) -> None:
        if self.variable_styles is None:
            return
        highlight_variables(text, self.variable_styles)


class VariablesAndUrlHighlighter(Highlighter):
    def __init__(self, input: Input) -> None:
        super().__init__()
        self.input = input
        self.variable_styles: VariableStyles = VariableStyles()
        self.url_styles: UrlStyles = UrlStyles()

    def highlight(self, text: Text) -> None:
        if text.plain == "":
            return

        highlight_url(text, self.url_styles)
        highlight_variables(text, self.variable_styles)

        input = self.input
        cursor_position = input.cursor_position  # type:ignore
        value: str = input.value

        if is_cursor_within_variable(cursor_position, value):  # type: ignore
            start = find_variable_start(cursor_position, value)  # type: ignore
            end = find_variable_end(cursor_position, value)  # type: ignore
            text.stylize("u", start, end)
