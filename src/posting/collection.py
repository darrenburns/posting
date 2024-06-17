from __future__ import annotations
import glob
from pathlib import Path
from typing import Literal
import httpx
from pydantic import BaseModel, Field
import yaml
import os


HttpRequestMethod = Literal["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"]


def str_presenter(dumper, data):
    if data.count("\n") > 0:
        data = "\n".join(
            [line.rstrip() for line in data.splitlines()]
        )  # Remove any trailing spaces, then put it back together again
        return dumper.represent_scalar("tag:yaml.org,2002:str", data, style="|")
    return dumper.represent_scalar("tag:yaml.org,2002:str", data)


yaml.add_representer(str, str_presenter)
yaml.representer.SafeRepresenter.add_representer(
    str, str_presenter
)  # to use with safe_dump


class Auth(BaseModel):
    type: str
    token: str


class Header(BaseModel):
    name: str
    value: str
    enabled: bool = Field(default=True)


class QueryParam(BaseModel):
    name: str
    value: str
    enabled: bool = Field(default=True)


class Cookie(BaseModel):
    name: str
    value: str
    enabled: bool = Field(default=True)

    @classmethod
    def from_httpx(cls, cookies: httpx.Cookies) -> list[Cookie]:
        return [Cookie(name=name, value=value) for name, value in cookies.items()]


class RequestModel(BaseModel):
    name: str | None = Field(default=None)
    """The name of the request. This is used to identify the request in the UI.
    Before saving a request, the name may be None."""

    description: str = Field(default="")
    """The description of the request."""

    method: HttpRequestMethod = Field(default="GET")
    """The HTTP method of the request."""

    url: str = Field()
    """The URL of the request."""

    path: Path | None = Field(default=None, exclude=True)
    """The path of the request on the file system (i.e. where the yaml is).
    Before saving a request, the path may be None."""

    body: str | None = Field(default=None)
    """The body of the request."""

    auth: Auth | None = Field(default=None)
    """The authentication information for the request."""

    headers: list[Header] = Field(default_factory=list)
    """The headers of the request."""

    params: list[QueryParam] = Field(default_factory=list)
    """The query parameters of the request."""

    cookies: list[Cookie] = Field(default_factory=list)
    """The cookies of the request."""

    posting_version: str = Field(default="1.0")
    """The version of Posting."""

    def to_httpx(self) -> httpx.Request:
        """Convert the request model to an httpx request."""
        return httpx.Request(
            method=self.method,
            url=self.url,
            content=self.body,
            headers=httpx.Headers(
                [
                    (header.name, header.value)
                    for header in self.headers
                    if header.enabled
                ]
            ),
            params=httpx.QueryParams(
                [(param.name, param.value) for param in self.params if param.enabled]
            ),
            cookies=httpx.Cookies(
                [
                    (cookie.name, cookie.value)
                    for cookie in self.cookies
                    if cookie.enabled
                ]
            ),
        )

    def save_to_disk(self, path: Path) -> None:
        """Save the request model to a YAML file."""
        content = self.model_dump(exclude_defaults=True, exclude_none=True)
        yaml_content = yaml.dump(content, None, sort_keys=False)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(yaml_content)


class Collection(BaseModel):
    path: Path
    name: str = Field(default="__default__")
    requests: list[RequestModel] = Field(default_factory=list)
    children: list[Collection] = Field(default_factory=list)

    @classmethod
    def from_directory(cls, directory: str | None = None) -> Collection:
        """Load all request models into a tree structure from a directory containing .posting.yaml files.

        Args:
            directory_path: The path to the directory containing .posting.yaml files.

        Returns:
            Collection: The root collection containing all loaded requests and subcollections.
        """
        if directory:
            directory_path = Path(directory)
        else:
            directory_path = Path.cwd()
            directory = str(directory_path)

        request_files = directory_path.rglob("*.posting.yaml")
        collection_name = directory_path.name
        root_collection = Collection(name=collection_name, path=directory_path)

        for file_path in request_files:
            try:
                file_path = str(file_path)
                request = load_request_from_yaml(file_path)
                path_parts = (
                    file_path[len(directory) :].strip(os.path.sep).split(os.path.sep)
                )
                current_level = root_collection
                for part in path_parts[:-1]:
                    found = False
                    for child in current_level.children:
                        if child.name == part:
                            current_level = child
                            found = True
                            break
                    if not found:
                        new_collection = Collection(name=part, path=directory_path)
                        current_level.children.append(new_collection)
                        current_level = new_collection
                current_level.requests.append(request)
            except Exception as e:
                print(f"Failed to load {file_path}: {e}")
        return root_collection


def load_request_from_yaml(file_path: str) -> RequestModel:
    """Load a request model from a YAML file.

    Args:
        file_path: The path to the YAML file.

    Returns:
        RequestModel: The request model loaded from the YAML file.
    """
    with open(file_path, "r") as file:
        data = yaml.safe_load(file)
        return RequestModel(**data, path=Path(file_path))


# Example usage
if __name__ == "__main__":
    sample_file_path = "sample-collections/users"
    collection = Collection.from_directory(sample_file_path)
    print(collection)
