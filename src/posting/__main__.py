from pathlib import Path
import tomllib
from typing import Any
import click

from click_default_group import DefaultGroup

from posting.app import Posting
from posting.collection import Collection
from posting.config import Settings
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
@click.option(
    "--collection",
    type=click.Path(exists=True),
    help="Path to the collection directory",
)
@click.option(
    "--env-file",
    type=click.Path(exists=True),
    help="Path to the .env environment file",
)
def default(collection: Path | None = None, env_file: Path | None = None) -> None:
    if collection:
        collection_tree = Collection.from_directory(str(collection))
    else:
        collection_tree = Collection.from_directory()

    collection_specified = collection is not None
    settings = Settings(_env_file=env_file)  # type: ignore[call-arg]
    app = Posting(settings, collection_tree, collection_specified)
    app.run()
