import getpass
import socket

from rich.text import Text

from posting.config import SETTINGS


def get_user_host_string() -> Text:
    try:
        username = getpass.getuser()
        hostname = SETTINGS.get().heading.hostname or socket.gethostname()
        return Text.from_markup(f"{username}@{hostname}")
    except Exception:
        return Text("unknown@unknown")
