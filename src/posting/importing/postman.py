from pathlib import Path
from typing import Any, List, Optional
import json
import os
import re
import yaml

from pydantic import BaseModel

from rich.console import Console

from posting.collection import APIInfo, Collection


class Variable(BaseModel):
    key: str
    value: Optional[str] = None
    src: Optional[str | List[str]] = None
    fileNotInWorkingDirectoryWarning: Optional[str] = None
    filesNotInWorkingDirectory: Optional[List[str]] = None
    type: Optional[str] = None
    disabled: Optional[bool] = None


class Body(BaseModel):
    mode: str
    options: Optional[dict] = None
    raw: Optional[str] = None
    formdata: Optional[List[Variable]] = None


class Url(BaseModel):
    raw: str
    host: Optional[List[str]] = None
    path: Optional[List[str]] = None
    query: Optional[List[Variable]] = None


class PostmanRequest(BaseModel):
    method: str
    url: Optional[str | Url] = None
    header: Optional[List[Variable]] = None
    description: Optional[str] = None
    body: Optional[Body] = None


class RequestItem(BaseModel):
    name: str
    item: Optional[List["RequestItem"]] = None
    request: Optional[PostmanRequest] = None


class PostmanCollection(BaseModel):
    info: dict[str, str]
    item: List[RequestItem]
    variable: List[Variable]


def create_env_file(path: Path, env_filename: str, variables: List[Variable]) -> Path:
    env_content: List[str] = []

    for var in variables:
        env_content.append(f"{transform_variables(var.key)}={var.value}")

    env_file = path / env_filename
    env_file.write_text("\n".join(env_content))
    return env_file


def generate_directory_structure(
    items: List[RequestItem], current_path: str = "", base_path: Path = Path("")
) -> List[str]:
    directories = []
    for item in items:
        if item.item is not None:
            folder_name = item.name
            new_path = f"{current_path}/{folder_name}" if current_path else folder_name
            full_path = Path(base_path) / new_path
            os.makedirs(str(full_path), exist_ok=True)
            directories.append(str(full_path))
            generate_directory_structure(item.item, new_path, base_path)
        if item.request is not None:
            request_name = re.sub(r"[^A-Za-z0-9\.]+", "", item.name)
            file_name = f"{request_name}.posting.yaml"
            full_path = Path(base_path) / current_path / file_name
            create_request_file(full_path, item)
    return directories


# Converts variable names like userId to $USER_ID, or user-id to $USER_ID
def transform_variables(string):
    underscore_case = re.sub(r"(?<!^)(?=[A-Z-])", "_", string).replace("-", "")
    return underscore_case.upper()


def transform_url(string):
    def replace_match(match):
        value = match.group(1)
        return f"${transform_variables(value)}"

    transformed = re.sub(r"\{\{([\w-]+)\}\}", replace_match, string)
    return transformed


def create_request_file(file_path: Path, request_data: RequestItem):
    yaml_content: dict[str, Any] = {
        "name": request_data.name,
    }

    if request_data.request is not None:
        yaml_content["method"] = request_data.request.method

        if request_data.request.header is not None:
            yaml_content["headers"] = [
                {"name": header.key, "value": header.value}
                for header in request_data.request.header
            ]

        if request_data.request.url is not None:
            if isinstance(request_data.request.url, Url):
                yaml_content["url"] = transform_url(request_data.request.url.raw)
                if request_data.request.url.query is not None:
                    yaml_content["params"] = [
                        {"name": param.key, "value": transform_url(param.value)}
                        for param in request_data.request.url.query
                    ]
            else:
                yaml_content["url"] = transform_url((request_data.request.url))

        if request_data.request.description is not None:
            yaml_content["description"] = request_data.request.description

        if (
            request_data.request.body is not None
            and request_data.request.body.raw is not None
        ):
            yaml_content["body"] = {
                "content": transform_url(request_data.request.body.raw)
            }

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
        spec_dict = json.load(file)

    spec = PostmanCollection(**spec_dict)

    info = APIInfo(
        title=spec.info["name"],
        description=spec.info.get("description", "No description"),
        specSchema=spec.info["schema"],
        version="2.0.0",
    )

    base_dir = spec_path.parent
    if output_path is not None:
        base_dir = Path(output_path) if isinstance(output_path, str) else output_path

    console.print(f"Output path: {output_path!r}")

    env_file = create_env_file(base_dir, f"{info.title}.env", spec.variable)
    console.print(f"Created environment file {str(env_file)!r}.")

    main_collection = Collection(path=spec_path.parent, name="Postman Test")

    # Create the directory structure like Postman's request folders
    directories = generate_directory_structure(spec.item, base_path=base_dir)
    console.print("Finished importing postman collection.")

    return main_collection, env_file, directories
