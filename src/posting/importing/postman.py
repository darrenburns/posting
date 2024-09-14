from pathlib import Path
from typing import List
import json
import os
import yaml

from rich.console import Console

from posting.collection import APIInfo, Collection


def create_env_file(
    path: Path, env_filename: str, variables: list[dict[str, str]]
) -> Path:
    env_content: list[str] = []

    for var in variables:
        key = var["key"]
        value = var["value"]
        env_content.append(f"{key!r}={value!r}")

    env_file = path / env_filename
    env_file.write_text("\n".join(env_content))
    return env_file


def generate_directory_structure(
    items: List[dict], current_path: str = "", base_path: Path = Path("")
) -> List[str]:
    directories = []
    for item in items:
        if "item" in item:
            folder_name = item["name"]
            new_path = f"{current_path}/{folder_name}" if current_path else folder_name
            full_path = Path(base_path) / new_path
            os.makedirs(str(full_path), exist_ok=True)
            directories.append(str(full_path))
            generate_directory_structure(item["item"], new_path, base_path)
        elif "request" in item:
            request_name = item["name"]
            file_name = f"{request_name}.posting.yaml"
            full_path = Path(base_path) / current_path / file_name
            create_request_file(full_path, item)
    return directories


def create_request_file(file_path: Path, request_data: dict):
    yaml_content = {
        "name": request_data["name"],
        "description": request_data.get("description", ""),
        "method": request_data["request"]["method"],
        "url": request_data["request"]["url"]["raw"],
    }

    # Add body if present
    if "body" in request_data["request"]:
        yaml_content["body"] = {
            "content": request_data["request"]["body"].get("raw", "")
        }

    # Add headers
    if "header" in request_data["request"]:
        yaml_content["headers"] = [
            {"name": header["key"], "value": header["value"]}
            for header in request_data["request"]["header"]
        ]

    # Add query params
    if "query" in request_data["request"]["url"]:
        yaml_content["params"] = [
            {"name": param["key"], "value": param["value"]}
            for param in request_data["request"]["url"]["query"]
        ]

    # Write YAML file
    with open(file_path, "w") as f:
        yaml.dump(yaml_content, f, default_flow_style=False)


def import_postman_spec(
    spec_path: str | Path, output_path: str | Path | None
) -> tuple[Collection, Path, List[str]]:
    console = Console()
    console.print(f"Importing Postman spec from {spec_path!r}.")

    spec_path = Path(spec_path)
    with open(spec_path, "r") as file:
        spec = json.load(file)

    info = APIInfo(
        title=spec["info"]["name"],
        description=spec["info"].get("description", "No description"),
        specSchema=spec["info"]["schema"],
        version="2.0.0",
    )

    base_dir = spec_path.parent
    if output_path is not None:
        base_dir = (
            Path(output_path).parent
            if isinstance(output_path, str)
            else output_path.parent
        )

    env_file = create_env_file(base_dir, f"{info.title}.env", spec["variable"])
    console.print(f"Created environment file {str(env_file)!r}.")

    main_collection = Collection(path=spec_path.parent, name="Postman Test")

    # Create the directory structure like Postman's request folders
    directories = generate_directory_structure(spec["item"], base_path=base_dir)
    console.print("Finished importing postman collection.")

    return main_collection, env_file, directories
