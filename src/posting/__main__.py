from pathlib import Path
import tomllib
from typing import Any
import click

from click_default_group import DefaultGroup

from posting.app import Posting
from posting.collection import Collection
from posting.locations import config_file, data_directory


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
def default(collection: Path | None = None) -> None:
    if collection:
        collection_tree = Collection.from_directory(str(collection))
    else:
        print(data_directory())
        default_collections_directory = data_directory() / "collections"
        collection_tree = Collection.from_directory(str(default_collections_directory))

    app = Posting(collection_tree)
    app.run()
