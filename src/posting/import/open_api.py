from __future__ import annotations

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


def import_openapi_spec(spec_path: str | Path) -> Collection:
    """
    Import an OpenAPI v3 specification and produce a Collection in memory.
    Creates subcollections for each server defined in the spec.

    Args:
        spec_path: The path to the OpenAPI specification file.

    Returns:
        A Collection object representing the imported OpenAPI specification.

    Raises:
        FileNotFoundError: If the specified file does not exist.
        yaml.YAMLError: If there's an error parsing the YAML file.
        ValueError: If the OpenAPI version is not 3.x.x.
    """
    spec_path = Path(spec_path)
    if not spec_path.exists():
        raise FileNotFoundError(f"OpenAPI specification file not found: {spec_path}")

    with open(spec_path, "r") as file:
        spec = yaml.safe_load(file)

    # Check OpenAPI version
    openapi_version = spec.get("openapi", "")
    if not openapi_version.startswith("3."):
        raise ValueError(
            f"Unsupported OpenAPI version: {openapi_version}. This function supports OpenAPI 3.x.x."
        )

    info = APIInfo(**spec.get("info", {}))
    external_docs = (
        ExternalDocs(**spec.get("externalDocs", {})) if "externalDocs" in spec else None
    )

    main_collection = Collection.from_openapi_spec(
        path=spec_path.parent, info=info, external_docs=external_docs
    )

    servers = spec.get("servers", [{"url": ""}])

    for server in servers:
        server_url = server.get("url", "")
        server_description = server.get("description", f"Server: {server_url}")

        server_collection = Collection(
            path=main_collection.path, name=server_description
        )

        for path, path_item in spec.get("paths", {}).items():
            for method, operation in path_item.items():
                if method.upper() not in VALID_HTTP_METHODS:
                    print(f"Skipping {method} {path} - invalid HTTP method.")
                    continue

                request = RequestModel(
                    name=operation.get("summary", f"{method.upper()} {path}"),
                    description=operation.get("description", ""),
                    method=method.upper(),
                    url=f"{server_url}{path}",
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

                server_collection.requests.append(request)

        main_collection.children.append(server_collection)

    return main_collection


if __name__ == "__main__":
    collection = import_openapi_spec("../posting-resources/petstore-expanded.yaml")
    from rich.console import Console
    from rich.markdown import Markdown

    console = Console()
    console.print(collection)

    console.print(Markdown(collection.readme or ""))
