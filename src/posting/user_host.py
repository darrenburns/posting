import getpass
import socket

from rich.text import Text


def get_user_host_string() -> Text:
    try:
        username = getpass.getuser()
        hostname = socket.gethostname()
        return Text(f"{username}@{hostname}")
    except Exception as e:
        print(f"Error getting user@host: {e}")
        return Text("unknown@unknown")
