from __future__ import annotations
import re
from typing import Any
from urllib.parse import urlparse
import json

import yaml
from openapi_pydantic import (
    OpenAPI,
    Reference,
    SecurityScheme,
    Operation,
    RequestBody as OpenAPIRequestBody,
    Schema,
    MediaType,
    DataType,
)
from pathlib import Path


from posting.collection import (
    VALID_HTTP_METHODS,
    APIInfo,
    Auth,
    BasicAuth,
    BearerTokenAuth,
    Collection,
    ExternalDocs,
    FormItem,
    Header,
    QueryParam,
    RequestBody,
    RequestModel,
)


from rich.console import Console


def resolve_url_variables(url: str, variables: dict[str, dict[str, str]]) -> str:
    """Resolve variables in the URL using their default values.

    Args:
        url: The URL to resolve.
        variables: A dictionary of server variables and their default values.

    Returns:
        str: The resolved URL.
    """
    for var, info in variables.items():
        url = url.replace(f"{{{var}}}", info["value"])
    return url


def generate_unique_env_filename(base_name: str, server_url: str) -> str:
    # Use the server URL to create a unique part of the filename
    parsed_url = urlparse(server_url)
    url_part = parsed_url.netloc + parsed_url.path

    # Slugify the URL part
    slugified_url = re.sub(r"[^\w\-_]", "_", url_part)

    # Trim if it's too long
    max_length = 50
    if len(slugified_url) > max_length:
        slugified_url = slugified_url[:max_length]

    # Remove trailing underscores
    slugified_url = slugified_url.rstrip("_")

    # TODO - if {id} is in the URL then it should be converted to the collection format.
    return f"{slugified_url}.env"


def extract_server_variables(spec: dict[str, Any]) -> dict[str, dict[str, str]]:
    variables: dict[str, dict[str, str]] = {}

    # Extract server URLs
    servers = spec.get("servers", [{"url": ""}])
    for i, server in enumerate(servers):
        var_name = f"SERVER_URL_{i}" if i > 0 else "BASE_URL"
        variables[var_name] = {
            "value": server.get("url", ""),
            "description": f"Server URL {i + 1}: {server.get('description', '')}",
        }

    return variables


def security_scheme_to_variables(
    name: str,
    security_scheme: SecurityScheme | Reference,
) -> dict[str, dict[str, str]]:
    match security_scheme:
        case SecurityScheme(type="http", scheme="basic"):
            return {
                f"{name.upper()}_USERNAME": {
                    "value": "YOUR USERNAME HERE",
                    "description": f"Username for {name} authentication",
                },
                f"{name.upper()}_PASSWORD": {
                    "value": "YOUR PASSWORD HERE",
                    "description": f"Password for {name} authentication",
                },
            }
        case SecurityScheme(type="http", scheme="bearer"):
            return {
                f"{name.upper()}_BEARER_TOKEN": {
                    "value": "YOUR BEARER TOKEN HERE",
                    "description": f"Token for {name} authentication",
                },
            }
        case _:
            return {}


def security_scheme_to_auth(
    name: str,
    security_scheme: SecurityScheme | Reference,
) -> Auth | None:
    match security_scheme:
        case SecurityScheme(type="http", scheme="basic"):
            return Auth(
                type="basic",
                basic=BasicAuth(
                    username=f"${{{name.upper()}_USERNAME}}",
                    password=f"${{{name.upper()}_PASSWORD}}",
                ),
            )
        case SecurityScheme(type="http", scheme="bearer"):
            return Auth(
                type="bearer_token",
                bearer_token=BearerTokenAuth(token=f"${{{name.upper()}_BEARER_TOKEN}}"),
            )
        case _:
            return None


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


def parse_schema_ref(ref: str, openapi: OpenAPI) -> Schema | None:
    if not openapi.components or not openapi.components.schemas:
        return None
    if not ref.startswith("#/components/schemas/"):
        return None
    ref_name = ref[len("#/components/schemas/") :]
    return openapi.components.schemas.get(ref_name)


class JsonBodyGenerator:
    def __init__(self, openapi: OpenAPI):
        self.openapi = openapi
        self.cache = {}
        self.seen = set()

    def generate_json(self, src: Reference | Schema | MediaType):
        obj = self.generate(src)
        if obj is None:
            return "{}"
        return json.dumps(obj, indent=2)

    def generate(self, src: Reference | Schema | MediaType):
        if isinstance(src, MediaType):
            if src.media_type_schema is None:
                return
            return self.generate(src.media_type_schema)

        if isinstance(src, Reference):
            ref = src.ref
            if ref in self.cache:
                return self.cache[ref]

            if ref in self.seen:
                return

            ref_schema = parse_schema_ref(ref, self.openapi)
            if ref_schema is None:
                return

            self.seen.add(ref)
            refobj = self._any_from_schema(ref_schema)
            self.seen.remove(ref)

            self.cache[ref] = refobj
            return refobj
        return self._any_from_schema(src)

    def _any_from_schema(self, schema: Schema):
        if schema.type == DataType.STRING:
            return schema.default or ""
        elif schema.type == DataType.NUMBER or schema.type == DataType.INTEGER:
            return schema.default or 0
        elif schema.type == DataType.BOOLEAN:
            return schema.default or False
        elif schema.type == DataType.ARRAY:
            if schema.items is None:
                return []
            item = self.generate(schema.items)
            if item is None:
                return []
            return [item]
        elif schema.type == DataType.OBJECT:
            obj = {}
            for name, schema in (schema.properties or {}).items():
                obj[name] = self.generate(schema)
            return obj


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
    )
    tag_collections: map[str, Collection] = {}

    openapi = OpenAPI.model_validate(spec)
    security_schemes = openapi.components.securitySchemes or {}

    env_files: list[Path] = []
    for server in servers:
        security_variables = {}
        for scheme_name, scheme in security_schemes.items():
            security_variables.update(security_scheme_to_variables(scheme_name, scheme))

        variables = {**extract_server_variables(server), **security_variables}
        env_filename = generate_unique_env_filename(collection_name, server["url"])
        env_file = create_env_file(spec_path.parent, env_filename, variables)
        console.print(
            f"Created environment file {str(env_file)!r} for server {server['url']!r}."
        )
        env_files.append(env_file)

    readme = generate_readme(spec_path, info, external_docs, servers, env_files)
    main_collection.readme = readme

    for path, path_item in (openapi.paths or {}).items():
        for method in path_item.model_fields_set:
            operation: Operation = getattr(path_item, method)
            method = method.upper()
            if method not in VALID_HTTP_METHODS:
                continue

            request = RequestModel(
                name=operation.summary or path.strip("/"),
                description=operation.description or "",
                method=method,
                url=f"${{BASE_URL}}{path}",
            )

            # Add auth
            for security in operation.security or []:
                for scheme_name, _scopes in security.items():
                    if scheme := security_schemes.get(scheme_name):
                        request.auth = security_scheme_to_auth(scheme_name, scheme)
                        break

            # Add query parameters
            for param in operation.parameters or []:
                if isinstance(param, Reference):
                    continue
                if param.param_in == "query":
                    request.params.append(
                        QueryParam(
                            name=param.name,
                            value="",  # Leave empty as it's just a template
                            enabled=not param.deprecated,
                        )
                    )

            # Add headers
            for param in operation.parameters or []:
                if param.param_in == "header":
                    request.headers.append(
                        Header(
                            name=param.name,
                            value="",  # Leave empty as it's just a template
                            enabled=not param.deprecated,
                        )
                    )

            # Add request body if present
            if isinstance(operation.requestBody, OpenAPIRequestBody):
                content = operation.requestBody.content
                if "application/json" in content:
                    request.body = RequestBody(
                        content=JsonBodyGenerator(openapi).generate_json(
                            content["application/json"]
                        )
                    )
                elif "application/x-www-form-urlencoded" in content:
                    form_data: list[FormItem] = []
                    body = content["application/x-www-form-urlencoded"]
                    for prop_name, _prop_schema in (
                        isinstance(body.media_type_schema, Schema)
                        and body.media_type_schema.properties
                        or {}
                    ).items():
                        form_data.append(FormItem(name=prop_name, value=""))
                    request.body = RequestBody(form_data=form_data)

            if operation.summary and operation.tags:
                tag = operation.tags[0]
                tag_collection = tag_collections.get(tag)
                if tag_collection is None:
                    tag_collection = Collection(
                        path=spec_path.parent,
                        name=tag,
                    )
                    tag_collections[tag] = tag_collection
                    main_collection.children.append(tag_collection)
                tag_collection.requests.append(request)
            else:
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
