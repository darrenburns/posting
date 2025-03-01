from pathlib import Path
import click

from click_default_group import DefaultGroup
from rich.console import Console

from posting.app import Posting
from posting.collection import Collection
from posting.config import Settings
from posting.importing.open_api import import_openapi_spec
from posting.locations import (
    config_file,
    default_collection_directory,
    theme_directory,
)
from posting.variables import load_variables


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
def default(collection: str | None = None, env: tuple[str, ...] = ()) -> None:
    create_config_file()
    default_collection = create_default_collection()
    collection_path = Path(collection) if collection else default_collection
    app = make_posting(
        collection=collection_path,
        env=env,
        using_default_collection=collection is None,
    )
    app.run()


@cli.command()
@click.argument(
    "thing_to_locate",
)
def locate(thing_to_locate: str) -> None:
    if thing_to_locate == "config":
        print("Config file:")
        print(config_file())
    elif thing_to_locate == "collection":
        print("Default collection directory:")
        print(default_collection_directory())
    elif thing_to_locate == "themes":
        print("Themes directory:")
        print(theme_directory())
    else:
        # This shouldn't happen because the type annotation should enforce that
        # the only valid options are "config" and "collection".
        print(f"Unknown thing to locate: {thing_to_locate!r}")


@cli.command(name="import")
@click.argument("spec_path", type=click.Path(exists=True))
@click.option(
    "--output",
    "-o",
    type=click.Path(),
    help="Path to save the imported collection",
    default=None,
)
def import_spec(spec_path: str, output: str | None) -> None:
    """Import an OpenAPI specification into a Posting collection."""
    console = Console()
    console.print(
        "Importing is currently an experimental feature.", style="bold yellow"
    )
    try:
        collection = import_openapi_spec(spec_path)

        if output:
            output_path = Path(output)
        else:
            output_path = Path(default_collection_directory()) / f"{collection.name}"

        output_path.mkdir(parents=True, exist_ok=True)
        collection.path = output_path
        collection.save_to_disk(output_path)

        console.print(f"Successfully imported OpenAPI spec to {str(output_path)!r}")
    except Exception:
        console.print("An error occurred during the import process.", style="red")
        console.print_exception()


def make_posting(
    collection: Path,
    env: tuple[str, ...] = (),
    using_default_collection: bool = False,
) -> Posting:
    """Return a Posting instance with the given collection and environment."""
    collection_tree = Collection.from_directory(str(collection.resolve()))

    env_paths = tuple(Path(e).resolve() for e in env)
    settings = Settings(_env_file=env_paths)  # type: ignore[call-arg]
    load_variables(env_paths, settings.use_host_environment)

    return Posting(settings, env_paths, collection_tree, not using_default_collection)


if __name__ == "__main__":
    cli()
