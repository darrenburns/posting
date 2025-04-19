"""The main entry point for the Posting CLI."""

from pathlib import Path
import click
import os

from click_default_group import DefaultGroup
from rich.console import Console

from posting.app import Posting
from posting.collection import Collection
from posting.config import Settings
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
    "-c",
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
@click.option(
    "--type", "-t", default="openapi", help="Specify spec type [openapi, postman]"
)
def import_spec(spec_path: str, output: str | None, type: str) -> None:
    """Import an OpenAPI specification into a Posting collection."""
    console = Console()
    console.print(
        "Importing is currently an experimental feature.", style="bold yellow"
    )

    output_path = None
    if output:
        output_path = Path(output)

    try:
        if type.lower() == "openapi":
            # We defer this import as it takes 64ms on an M4 MacBook Pro,
            # and is only needed for a single CLI command - not for the main TUI.
            from posting.importing.open_api import import_openapi_spec

            spec_type = "OpenAPI"
            collection = import_openapi_spec(spec_path)
            if output_path is None:
                output_path = (
                    Path(default_collection_directory()) / f"{collection.name}"
                )

            output_path.mkdir(parents=True, exist_ok=True)
            collection.path = output_path
            collection.save_to_disk(output_path)
        elif type.lower() == "postman":
            from posting.importing.postman import import_postman_spec, create_env_file

            spec_type = "Postman"
            collection, postman_collection = import_postman_spec(spec_path, output)
            if output_path is None:
                output_path = (
                    Path(default_collection_directory()) / f"{collection.name}"
                )

            output_path.mkdir(parents=True, exist_ok=True)
            collection.path = output_path
            collection.save_to_disk(output_path)

            # Create the environment file in the collection directory.
            env_file = create_env_file(
                output_path, f"{collection.name}.env", postman_collection.variable
            )
            console.print(f"Created environment file {str(env_file)!r}.")
        else:
            console.print(f"Unknown spec type: {type!r}", style="red")
            return

        console.print(
            f"Successfully imported {spec_type!r} spec to {str(output_path)!r}"
        )
    except Exception:
        console.print("An error occurred during the import process.", style="red")
        console.print(
            "Ensure you're importing the correct type of collection.", style="red"
        )
        console.print(
            "For Postman collections, use `posting import --type postman <path>`",
            style="red",
        )
        console.print(
            "No luck? Please include the traceback below in your issue report.",
            style="red",
        )
        console.print_exception()


@cli.command(name="sponsors")
def sponsors() -> None:
    """Show the list of sponsors."""
    print("Thanks to everyone below who contributed to the development of Posting 🚀\n")

    # Sponsors are added to the list below, name on the left, description on the right
    sponsors = [
        ("Michael Howard", "https://github.com/elithper"),
    ]
    for sponsor in sponsors:
        print(f"{sponsor[0]} - {sponsor[1]}")

    print()
    print("If you'd like to sponsor the development of Posting, please visit:")
    print("https://github.com/sponsors/darrenburns")


def make_posting(
    collection: Path,
    env: tuple[str, ...] = (),
    using_default_collection: bool = False,
) -> Posting:
    """Return a Posting instance with the given collection and environment."""
    collection_tree = Collection.from_directory(str(collection.resolve()))

    # if env empty then load current directory posting.env file if it exists
    if not env and os.path.exists("posting.env"):
        env = ("posting.env",)

    env_paths = tuple(Path(e).resolve() for e in env)
    settings = Settings(_env_file=env_paths)  # type: ignore[call-arg]
    load_variables(env_paths, settings.use_host_environment)

    return Posting(settings, env_paths, collection_tree, not using_default_collection)


if __name__ == "__main__":
    cli()
