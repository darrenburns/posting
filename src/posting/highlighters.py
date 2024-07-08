import re
from rich.highlighter import Highlighter
from rich.text import Text

from posting.variables import find_variables, get_variables


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
            text.stylize("dim", index, index + 1)


class URLHighlighter(Highlighter):
    def highlight(self, text: Text) -> None:
        highlight_url(text)


def highlight_variables(text: Text) -> None:
    for match in find_variables(text.plain):
        variable_name, start, end = match
        if variable_name not in get_variables():
            text.stylize("dim", start, end)
        else:
            text.stylize("green not dim", start, end)


class VariableHighlighter(Highlighter):
    def highlight(self, text: Text) -> None:
        highlight_variables(text)


class VariablesAndUrlHighlighter(Highlighter):
    def highlight(self, text: Text) -> None:
        highlight_url(text)
        highlight_variables(text)
