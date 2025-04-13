from __future__ import annotations

from pathlib import Path
import json
import re
from urllib.parse import urlparse, urlunparse

from pydantic import BaseModel, Field

from rich.console import Console

from posting.collection import (
    APIInfo,
    Collection,
    FormItem,
    Header,
    QueryParam,
    RequestBody,
    RequestModel,
    HttpRequestMethod,
)


class Variable(BaseModel):
    key: str
    value: str | None = None
    src: str | list[str] | None = None
    fileNotInWorkingDirectoryWarning: str | None = None
    filesNotInWorkingDirectory: list[str] | None = None
    type: str | None = None
    disabled: bool | None = None


class RawRequestOptions(BaseModel):
    language: str


class RequestOptions(BaseModel):
    raw: RawRequestOptions


class Body(BaseModel):
    mode: str
    options: RequestOptions | None = None
    raw: str | None = None
    formdata: list[Variable] | None = None


class Url(BaseModel):
    raw: str
    host: list[str] | None = None
    path: list[str] | None = None
    query: list[Variable] | None = None


class PostmanRequest(BaseModel):
    method: HttpRequestMethod
    url: str | Url | None = None
    header: list[Variable] | None = None
    description: str | None = None
    body: Body | None = None


class RequestItem(BaseModel):
    name: str
    item: list["RequestItem"] | None = None
    request: PostmanRequest | None = None


class PostmanCollection(BaseModel):
    info: dict[str, str] = Field(default_factory=dict)
    variable: list[Variable] = Field(default_factory=list)
    item: list[RequestItem]


# Converts variable names like userId to $USER_ID, or user-id to $USER_ID
def sanitize_variables(string: str) -> str:
    underscore_case = re.sub(r"(?<!^)(?=[A-Z-])", "_", string).replace("-", "")
    return underscore_case.upper()


def sanitize_str(string: str) -> str:
    def replace_match(match: re.Match[str]) -> str:
        value = match.group(1)
        return f"${sanitize_variables(value)}"

    transformed = re.sub(r"\{\{([\w-]+)\}\}", replace_match, string)
    return transformed


def create_env_file(path: Path, env_filename: str, variables: list[Variable]) -> Path:
    env_content: list[str] = []

    for var in variables:
        env_content.append(f"{sanitize_variables(var.key)}={var.value}")

    env_file = path / env_filename
    env_file.write_text("\n".join(env_content))
    return env_file


def format_request(name: str, request: PostmanRequest) -> RequestModel:
    # Extract the raw URL first
    raw_url_with_query: str = ""
    if request.url is not None:
        raw_url_with_query = (
            request.url.raw if isinstance(request.url, Url) else request.url
        )

    # Parse the URL and remove query parameters
    parsed_url = urlparse(raw_url_with_query)
    # Reconstruct the URL without the query string
    url_without_query = urlunparse(
        (
            parsed_url.scheme,
            parsed_url.netloc,
            parsed_url.path,
            parsed_url.params,  # Keep fragment/params if they exist
            "",  # Empty query string
            parsed_url.fragment,
        )
    )
    sanitized_url = sanitize_str(url_without_query)

    posting_request = RequestModel(
        name=name,
        method=request.method,
        description=request.description if request.description is not None else "",
        url=sanitized_url,
    )

    if request.header is not None:
        for header in request.header:
            posting_request.headers.append(
                Header(
                    name=header.key,
                    value=header.value if header.value is not None else "",
                    enabled=True,
                )
            )

    # Add query params to the request (they've been removed from the URL)
    if (
        request.url is not None
        and isinstance(request.url, Url)
        and request.url.query is not None
    ):
        for param in request.url.query:
            posting_request.params.append(
                QueryParam(
                    name=param.key,
                    value=param.value if param.value is not None else "",
                    enabled=param.disabled if param.disabled is not None else True,
                )
            )

    if request.body is not None and request.body.raw is not None:
        if (
            request.body.mode == "raw"
            and request.body.options is not None
            and request.body.options.raw.language == "json"
        ):
            posting_request.body = RequestBody(content=sanitize_str(request.body.raw))
        elif request.body.mode == "formdata" and request.body.formdata is not None:
            form_data: list[FormItem] = [
                FormItem(
                    name=data.key,
                    value=data.value if data.value is not None else "",
                    enabled=data.disabled is False,
                )
                for data in request.body.formdata
            ]
            posting_request.body = RequestBody(form_data=form_data)

    return posting_request


def process_item(
    item: RequestItem, parent_collection: Collection, base_path: Path
) -> None:
    if item.item is not None:
        # This is a folder - create a subcollection
        child_path = base_path / item.name
        child_collection = Collection(path=child_path, name=item.name)
        parent_collection.children.append(child_collection)

        # Process items in this folder
        for sub_item in item.item:
            process_item(sub_item, child_collection, child_path)

    if item.request is not None:
        # This is a request - add it to the current collection
        file_name = "".join(
            word.capitalize()
            for word in re.sub(r"[^A-Za-z0-9\.]+", " ", item.name).split()
        )
        request = format_request(item.name, item.request)
        request_path = parent_collection.path / f"{file_name}.posting.yaml"
        request.path = request_path
        parent_collection.requests.append(request)


def import_postman_spec(
    spec_path: str | Path, output_path: str | Path | None
) -> tuple[Collection, PostmanCollection]:
    """Import a Postman collection from a file and save it to disk."""
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

    base_dir.mkdir(parents=True, exist_ok=True)
    main_collection = Collection(path=base_dir, name=info.title)
    main_collection.readme = main_collection.generate_readme(info)

    for item in spec.item:
        process_item(item, main_collection, base_dir)

    return main_collection, spec
