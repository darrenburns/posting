import getpass
import socket

from rich.text import Text


def get_user_host_string() -> Text:
    try:
        username = getpass.getuser()
        hostname = socket.gethostname()
        return Text(f"{username}@{hostname}")
    except Exception:
        return Text("unknown@unknown")
