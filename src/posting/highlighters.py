import re
from rich.highlighter import Highlighter
from rich.text import Text
from textual.widgets import Input
from textual.geometry import clamp

from posting.variables import (
    find_variable_end,
    find_variable_start,
    find_variables,
    get_variable_at_cursor,
    get_variables,
    is_cursor_within_variable,
)


_URL_REGEX = re.compile(r"(?P<protocol>https?)://(?P<base>[^/]+)(?P<path>/[^ ]*)?")


def highlight_url(text: Text) -> None:
    for match in _URL_REGEX.finditer(text.plain):
        protocol_start, protocol_end = match.span("protocol")
        base_start, base_end = match.span("base")
        separator_start, separator_end = protocol_end, protocol_end + 3

        text.stylize("#818cf8", protocol_start, protocol_end)
        text.stylize("dim", separator_start, separator_end)
        text.stylize("#00C168", base_start, base_end)

    for index, char in enumerate(text.plain):
        if char == "/":
            text.stylize("dim b", index, index + 1)


class URLHighlighter(Highlighter):
    def highlight(self, text: Text) -> None:
        highlight_url(text)


def highlight_variables(text: Text) -> None:
    for match in find_variables(text.plain):
        variable_name, start, end = match
        if variable_name not in get_variables():
            text.stylize("dim", start, end)
        else:
            text.stylize("b green not dim", start, end)


class VariableHighlighter(Highlighter):
    def highlight(self, text: Text) -> None:
        highlight_variables(text)


class VariablesAndUrlHighlighter(Highlighter):
    def __init__(self, input: Input) -> None:
        super().__init__()
        self.input = input

    def highlight(self, text: Text) -> None:
        if text.plain == "":
            return

        highlight_url(text)
        highlight_variables(text)
        input = self.input
        cursor_position = input.cursor_position  # type:ignore
        value: str = input.value

        if is_cursor_within_variable(cursor_position, value):  # type: ignore
            start = find_variable_start(cursor_position, value)  # type: ignore
            end = find_variable_end(cursor_position, value)  # type: ignore
            text.stylize("u", start, end)
