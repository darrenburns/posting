from pathlib import Path
from typing import List, Optional
import json
import re

from pydantic import BaseModel

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
    value: Optional[str] = None
    src: Optional[str | List[str]] = None
    fileNotInWorkingDirectoryWarning: Optional[str] = None
    filesNotInWorkingDirectory: Optional[List[str]] = None
    type: Optional[str] = None
    disabled: Optional[bool] = None


class RawRequestOptions(BaseModel):
    language: str


class RequestOptions(BaseModel):
    raw: RawRequestOptions


class Body(BaseModel):
    mode: str
    options: Optional[RequestOptions] = None
    raw: Optional[str] = None
    formdata: Optional[List[Variable]] = None


class Url(BaseModel):
    raw: str
    host: Optional[List[str]] = None
    path: Optional[List[str]] = None
    query: Optional[List[Variable]] = None


class PostmanRequest(BaseModel):
    method: HttpRequestMethod
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
    variable: List[Variable]

    item: List[RequestItem]


# Converts variable names like userId to $USER_ID, or user-id to $USER_ID
def sanitize_variables(string):
    underscore_case = re.sub(r"(?<!^)(?=[A-Z-])", "_", string).replace("-", "")
    return underscore_case.upper()


def sanitize_str(string):
    def replace_match(match):
        value = match.group(1)
        return f"${sanitize_variables(value)}"

    transformed = re.sub(r"\{\{([\w-]+)\}\}", replace_match, string)
    return transformed


def create_env_file(path: Path, env_filename: str, variables: List[Variable]) -> Path:
    env_content: List[str] = []

    for var in variables:
        env_content.append(f"{sanitize_variables(var.key)}={var.value}")

    env_file = path / env_filename
    env_file.write_text("\n".join(env_content))
    return env_file


def import_requests(
    items: List[RequestItem], base_path: Path = Path("")
) -> List[RequestModel]:
    requests: List[RequestModel] = []
    for item in items:
        if item.item is not None:
            requests = requests + import_requests(item.item, base_path)
        if item.request is not None:
            file_name = re.sub(r"[^A-Za-z0-9\.]+", "", item.name)
            requests.append(format_request(file_name, item.request))

    return requests


def format_request(name: str, request: PostmanRequest) -> RequestModel:
    postingRequest = RequestModel(
        name=name,
        method=request.method,
        description=request.description if request.description is not None else "",
        url=sanitize_str(
            request.url.raw if isinstance(request.url, Url) else request.url
        )
        if request.url is not None
        else "",
    )

    if request.header is not None:
        for header in request.header:
            postingRequest.headers.append(
                Header(
                    name=header.key,
                    value=header.value if header.value is not None else "",
                    enabled=True,
                )
            )

    if (
        request.url is not None
        and isinstance(request.url, Url)
        and request.url.query is not None
    ):
        for param in request.url.query:
            postingRequest.params.append(
                QueryParam(
                    name=param.key,
                    value=param.value if param.value is not None else "",
                    enabled=param.disabled if param.disabled is not None else False,
                )
            )

    if request.body is not None and request.body.raw is not None:
        if (
            request.body.mode == "raw"
            and request.body.options is not None
            and request.body.options.raw.language == "json"
        ):
            postingRequest.body = RequestBody(content=sanitize_str(request.body.raw))
        elif request.body.mode == "formdata" and request.body.formdata is not None:
            form_data: list[FormItem] = [
                FormItem(
                    name=data.key,
                    value=data.value if data.value is not None else "",
                    enabled=data.disabled is False,
                )
                for data in request.body.formdata
            ]
            postingRequest.body = RequestBody(form_data=form_data)

    return postingRequest


def import_postman_spec(
    spec_path: str | Path, output_path: str | Path | None
) -> Collection:
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

    console.print(f"Output path: {str(base_dir)!r}")

    env_file = create_env_file(base_dir, f"{info.title}.env", spec.variable)
    console.print(f"Created environment file {str(env_file)!r}.")

    main_collection = Collection(path=spec_path.parent, name=info.title)
    main_collection.readme = main_collection.generate_readme(info)

    main_collection.requests = import_requests(spec.item, base_dir)

    return main_collection
