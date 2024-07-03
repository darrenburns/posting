from pathlib import Path
import click

from click_default_group import DefaultGroup
from dotenv import load_dotenv

from posting.app import Posting
from posting.collection import Collection
from posting.config import Settings
from posting.locations import config_file, default_collection_directory


def create_config_file() -> None:
    f = config_file()
    if f.exists():
        return

    try:
        f.touch()
    except OSError:
        pass


def create_default_collection() -> Path:
    f = default_collection_directory()
    if f.exists():
        return f

    try:
        f.mkdir(parents=True)
    except OSError:
        pass

    return f


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
    "--env",
    "-e",
    type=click.Path(exists=True),
    help="Path to the .env environment file(s)",
    multiple=True,
)
def default(collection: Path | None = None, env: tuple[Path, ...] = ()) -> None:
    create_config_file()
    default_collection = create_default_collection()

    if collection:
        collection_tree = Collection.from_directory(str(collection))
    else:
        collection_tree = Collection.from_directory(str(default_collection))

    collection_specified = collection is not None
    settings = Settings(_env_file=env)  # type: ignore[call-arg]

    app = Posting(settings, env, collection_tree, collection_specified)
    app.run()


@cli.command()
@click.argument(
    "thing_to_locate",
    type=click.Choice(["config", "collection"]),
)
def locate(thing_to_locate: str) -> None:
    if thing_to_locate == "config":
        print("Config file:")
        print(config_file())
    elif thing_to_locate == "collection":
        print("Default collection directory:")
        print(default_collection_directory())
    else:
        # This shouldn't happen because the type annotation should enforce that
        # the only valid options are "config" and "collection".
        raise ValueError(f"Unknown thing to locate: {thing_to_locate}")
