import tomllib
from typing import Any
import click

from click_default_group import DefaultGroup

from posting.app import Posting
from posting.locations import config_file


def load_or_create_config_file() -> dict[str, Any]:
    config = config_file()

    try:
        file_config = tomllib.loads(config.read_text())
    except FileNotFoundError:
        file_config = {}
        try:
            config.touch()
        except OSError:
            pass

    return file_config


@click.group(cls=DefaultGroup, default="default", default_if_no_args=True)
def cli() -> None:
    """A TUI for testing HTTP APIs."""


@cli.command()
def default() -> None:
    app = Posting()
    app.run()
