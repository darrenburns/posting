import re
from rich.highlighter import Highlighter
from rich.text import Text


class URLHighlighter(Highlighter):
    def highlight(self, text: Text) -> None:
        url_regex = re.compile(
            r"(?P<protocol>https?)://(?P<base>[^/]+)(?P<path>/[^ ]*)?"
        )
        for match in url_regex.finditer(text.plain):
            protocol_start, protocol_end = match.span("protocol")
            base_start, base_end = match.span("base")
            separator_start, separator_end = protocol_end, protocol_end + 3

            text.stylize("#818cf8", protocol_start, protocol_end)
            text.stylize("dim", separator_start, separator_end)
            text.stylize("#00C168", base_start, base_end)
