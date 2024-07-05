from __future__ import annotations
import re
from typing import Any
from urllib.parse import urlparse

import yaml
from pathlib import Path


from posting.collection import (
    VALID_HTTP_METHODS,
    APIInfo,
    Collection,
    ExternalDocs,
    FormItem,
    Header,
    QueryParam,
    RequestBody,
    RequestModel,
)


from urllib.parse import urlparse
from rich.console import Console


def generate_unique_env_filename(
    base_name: str, server_url: str, variables: dict[str, dict[str, str]]
) -> str:
    """
    Generate a unique .env filename by appending a slugified server URL to the base name,
    considering potential variables in the URL.

    Args:
        base_name (str): The base name for the .env file (typically the collection name).
        server_url (str): The URL of the server, potentially containing variables.
        variables (dict): A dictionary of server variables and their default values.

    Returns:
        str: A unique .env filename.
    """
    # Replace variables in the URL with their default values
    for var, value in variables.items():
        server_url = server_url.replace(f"{{{var}}}", value.get("value", ""))

    # Parse the server URL
    parsed_url = urlparse(server_url)

    # Extract hostname and path
    hostname = parsed_url.hostname or ""
    path = parsed_url.path.strip("/")

    # Combine hostname and path
    url_part = f"{hostname}_{path}" if path else hostname

    # Slugify the URL part
    slugified_url = re.sub(r"[^\w\-_]", "_", url_part)

    # Trim the slugified URL if it's too long
    max_slug_length = 50
    if len(slugified_url) > max_slug_length:
        slugified_url = slugified_url[:max_slug_length]

    # Remove trailing underscores
    slugified_url = slugified_url.rstrip("_")

    # Combine base name and slugified URL
    return f"{slugified_url}.env"


def extract_server_variables(server: dict[str, Any]) -> dict[str, dict[str, str]]:
    variables = {
        "BASE_URL": {
            "value": server.get("url", ""),
            "description": "The base URL for this server",
        }
    }
    server_variables = server.get("variables", {})

    for var, var_info in server_variables.items():
        variables[var] = {
            "value": var_info.get("default", f"PLACEHOLDER_{var.upper()}"),
            "description": var_info.get("description", ""),
        }
        if "enum" in var_info and var_info["enum"] and "default" not in var_info:
            variables[var]["value"] = var_info["enum"][
                0
            ]  # Use the first enum value if no default

    return variables


def generate_readme(
    spec_path: Path,
    info: APIInfo,
    external_docs: ExternalDocs | None,
    servers: list[dict[str, Any]],
    env_files: list[Path],
) -> str:
    readme = f"# {info.title}\n\n"
    readme += f"Imported from `{spec_path.name}`.\n\n"

    if info.description:
        readme += f"{info.description}\n\n"

    readme += f"Version: {info.version}\n\n"

    if info.termsOfService:
        readme += f"Terms of Service: {info.termsOfService}\n\n"

    if info.contact:
        readme += "## Contact Information\n"
        if info.contact.name:
            readme += f"Name: {info.contact.name}\n\n"
        if info.contact.email:
            readme += f"Email: {info.contact.email}\n\n"
        if info.contact.url:
            readme += f"URL: {info.contact.url}\n\n"
        readme += "\n"

    if info.license:
        readme += "## License\n"
        readme += f"Name: {info.license.name}\n\n"
        if info.license.url:
            readme += f"URL: {info.license.url}\n\n"
        readme += "\n"

    if external_docs:
        readme += "## External Documentation\n"
        if external_docs.description:
            readme += f"{external_docs.description}\n\n"
        readme += f"URL: {external_docs.url}\n\n"

    readme += "## Servers\n"
    readme += "A separate `.env` file is generated for each server.\n\n"
    for server in servers:
        readme += f"- {server.get('url', 'No URL')} ({server.get('description', 'No description')})\n"
    readme += "\n"
    readme += "Environment variables are stored in the following files:\n"
    for env_file in env_files:
        readme += f"- `{env_file.name}`\n\n"

    readme += "To load an environment run `posting` with the `--env` option, passing the path of the file."

    return readme


def create_env_file(
    path: Path, env_filename: str, variables: dict[str, dict[str, str]]
) -> Path:
    env_content: list[str] = []
    for var, info in variables.items():
        if info["description"]:
            env_content.append(f"# {info['description']}")

        # Ensure the value is properly quoted if it contains spaces or special characters
        value = info["value"].replace('"', '\\"')  # Escape any existing double quotes
        if " " in value or any(char in value for char in "'\"\\"):
            value = f'"{value}"'

        env_content.append(f"{var}={value}")
        env_content.append("")  # Add a blank line after each variable for readability

    env_file = path / env_filename
    env_file.write_text("\n".join(env_content))
    return env_file


def import_openapi_spec(spec_path: str | Path) -> Collection:
    console = Console()
    console.print(f"Importing OpenAPI spec from {spec_path!r}.")
    spec_path = Path(spec_path)
    with open(spec_path, "r") as file:
        spec = yaml.safe_load(file)

    info = APIInfo(**spec.get("info", {}))
    external_docs = (
        ExternalDocs(**spec.get("externalDocs", {})) if "externalDocs" in spec else None
    )

    collection_name = spec_path.stem
    servers = spec.get("servers", [{"url": ""}])

    main_collection = Collection(
        path=spec_path.parent,
        name=collection_name,
        readme="",  # We'll update this later
    )

    env_files: list[Path] = []
    for server in servers:
        variables = extract_server_variables(server)
        env_filename = generate_unique_env_filename(
            collection_name, server["url"], variables
        )
        env_file = create_env_file(
            spec_path.parent,
            env_filename,
            variables,
        )
        console.print(
            f"Created environment file {str(env_file)!r} for server {server['url']!r}."
        )
        env_files.append(env_file)

    readme = generate_readme(spec_path, info, external_docs, servers, env_files)
    main_collection.readme = readme

    for path, path_item in spec.get("paths", {}).items():
        for method, operation in path_item.items():
            method = method.upper()
            if method not in VALID_HTTP_METHODS:
                continue

            request = RequestModel(
                name=operation.get("summary", path.strip("/")),
                description=operation.get("description", ""),
                method=method,
                url=f"${{env:BASE_URL}}{path}",
            )
            # Add query parameters
            for param in operation.get("parameters", []):
                if param["in"] == "query":
                    request.params.append(
                        QueryParam(
                            name=param["name"],
                            value="",  # Leave empty as it's just a template
                            enabled=not param.get("deprecated", False),
                        )
                    )

            # Add headers
            for param in operation.get("parameters", []):
                if param["in"] == "header":
                    request.headers.append(
                        Header(
                            name=param["name"],
                            value="",  # Leave empty as it's just a template
                            enabled=not param.get("deprecated", False),
                        )
                    )

            # Add request body if present
            if "requestBody" in operation:
                content = operation["requestBody"].get("content", {})
                if "application/json" in content:
                    request.body = RequestBody(
                        content="{}"
                    )  # Empty JSON object as template
                elif "application/x-www-form-urlencoded" in content:
                    form_data: list[FormItem] = []
                    for prop_name, _prop_schema in (
                        content["application/x-www-form-urlencoded"]
                        .get("schema", {})
                        .get("properties", {})
                        .items()
                    ):
                        form_data.append(FormItem(name=prop_name, value=""))
                    request.body = RequestBody(form_data=form_data)

            main_collection.requests.append(request)

    console.print(f"Imported {len(main_collection.requests)} requests.")
    return main_collection


if __name__ == "__main__":
    collection = import_openapi_spec("../posting-resources/petstore-expanded.yaml")
    from rich.console import Console
    from rich.markdown import Markdown

    console = Console()
    console.print(collection)

    console.print(Markdown(collection.readme or ""))
