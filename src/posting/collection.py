from __future__ import annotations
from pathlib import Path
from typing import Literal
import httpx
from pydantic import BaseModel, Field
import yaml
import os
import glob

HttpRequestMethod = Literal["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"]


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


class RequestModel(BaseModel):
    name: str
    path: Path
    """The path of the request on the file system (i.e. where the yaml is)"""
    url: str
    method: HttpRequestMethod = Field(default="GET")
    description: str = Field(default="")
    body: str | None = Field(default=None)
    auth: Auth | None = Field(default=None)
    headers: list[Header] = Field(default_factory=list)
    params: list[QueryParam] = Field(default_factory=list)
    posting_version: str = Field(default="1.0")

    def to_httpx_request(self) -> httpx.Request:
        headers = httpx.Headers(
            [(header.name, header.value) for header in self.headers]
        )
        params = httpx.QueryParams([(param.name, param.value) for param in self.params])

        return httpx.Request(
            method=self.method, url=self.url, headers=headers, params=params
        )


class Collection(BaseModel):
    path: Path
    name: str = Field(default="__default__")
    requests: list[RequestModel] = Field(default_factory=list)
    children: list[Collection] = Field(default_factory=list)

    @classmethod
    def from_directory(cls, directory: str) -> Collection:
        """Load all request models into a tree structure from a directory containing .posting.yaml files.

        Args:
            directory_path: The path to the directory containing .posting.yaml files.

        Returns:
            Collection: The root collection containing all loaded requests and subcollections.
        """
        directory_path = Path(directory)
        request_files = glob.glob(
            str(directory_path / "**/*.posting.yaml"), recursive=True
        )
        collection_name = directory_path.name

        root_collection = Collection(name=collection_name, path=directory_path)

        for file_path in request_files:
            try:
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
