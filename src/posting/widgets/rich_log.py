from io import StringIO
from typing import Literal
from textual.widgets import RichLog


class RichLogIO(StringIO):
    def __init__(self, rich_log: RichLog, stream_type: Literal["stdout", "stderr"]):
        super().__init__()
        self.rich_log: RichLog = rich_log
        self.stream_type: Literal["stdout", "stderr"] = stream_type
        self._buffer: str = ""

    def write(self, s: str) -> int:
        lines = (self._buffer + s).splitlines(True)
        self._buffer = ""
        for line in lines:
            if line.endswith("\n"):
                self._flush_line(line.rstrip("\n"))
            else:
                self._buffer = line
        return len(s)

    def _flush_line(self, line: str) -> None:
        if self.stream_type == "stdout":
            self.rich_log.write(f" [green]out[/green] {line}")
        else:
            self.rich_log.write(f" [red]err[/red] {line}")

    def flush(self) -> None:
        if self._buffer:
            self._flush_line(self._buffer)
            self._buffer = ""
        super().flush()
