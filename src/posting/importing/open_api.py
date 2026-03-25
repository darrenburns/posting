from __future__ import annotations
import re
from typing import Any, NamedTuple
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


class OpenAPIModels(NamedTuple):
    OpenAPI: type
    Reference: type
    SecurityScheme: type
    Operation: type
    RequestBody: type
    Schema: type
    MediaType: type
    DataType: type


def _get_openapi_models(version: str) -> OpenAPIModels:
    """Return the correct set of OpenAPI pydantic models for the given spec version."""
    if version.startswith("3.0"):
        from openapi_pydantic.v3.v3_0 import (
            OpenAPI as _OpenAPI,
            Reference as _Reference,
            SecurityScheme as _SecurityScheme,
            Operation as _Operation,
            RequestBody as _RequestBody,
            Schema as _Schema,
            MediaType as _MediaType,
            DataType as _DataType,
        )
        return OpenAPIModels(_OpenAPI, _Reference, _SecurityScheme, _Operation, _RequestBody, _Schema, _MediaType, _DataType)
    elif version.startswith("3.1"):
        return OpenAPIModels(OpenAPI, Reference, SecurityScheme, Operation, OpenAPIRequestBody, Schema, MediaType, DataType)
    else:
        raise ValueError(
            f"Unsupported OpenAPI version: {version!r}. Only 3.0.x and 3.1.x are supported."
        )


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


def extract_server_variables(server: dict[str, Any]) -> dict[str, dict[str, str]]:
    server_variables = {
        name: {
            "value": str(info.get("default", "")),
            "description": str(info.get("description", "")),
        }
        for name, info in (server.get("variables") or {}).items()
        if isinstance(info, dict)
    }
    return {
        "BASE_URL": {
            "value": resolve_url_variables(str(server.get("url", "")), server_variables),
            "description": f"Server URL: {server.get('description', '')}",
        }
    }


def security_scheme_to_variables(
    name: str,
    security_scheme: SecurityScheme | Reference,
    security_scheme_cls: type = SecurityScheme,
) -> dict[str, dict[str, str]]:
    match security_scheme:
        case security_scheme_cls(type="http", scheme="basic"):
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
        case security_scheme_cls(type="http", scheme="bearer"):
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
    security_scheme_cls: type = SecurityScheme,
) -> Auth | None:
    match security_scheme:
        case security_scheme_cls(type="http", scheme="basic"):
            return Auth(
                type="basic",
                basic=BasicAuth(
                    username=f"${{{name.upper()}_USERNAME}}",
                    password=f"${{{name.upper()}_PASSWORD}}",
                ),
            )
        case security_scheme_cls(type="http", scheme="bearer"):
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


def parse_component_ref(
    ref: str,
    openapi: OpenAPI,
    component_name: str,
    reference_cls: type = Reference,
    seen: set[str] | None = None,
):
    components = getattr(openapi, "components", None)
    component_map = getattr(components, component_name, None) if components else None
    if not component_map:
        return None

    prefix = f"#/components/{component_name}/"
    if not ref.startswith(prefix):
        return None
    ref_name = ref[len(prefix) :]
    component = component_map.get(ref_name)
    if component is None:
        return None
    if isinstance(component, reference_cls):
        nested_ref = component.ref
        seen = seen or set()
        if nested_ref in seen:
            return None
        seen.add(nested_ref)
        return parse_component_ref(
            nested_ref,
            openapi,
            component_name,
            reference_cls=reference_cls,
            seen=seen,
        )
    return component


def parse_schema_ref(
    ref: str,
    openapi: OpenAPI,
    reference_cls: type = Reference,
) -> Schema | None:
    return parse_component_ref(ref, openapi, "schemas", reference_cls)


def resolve_parameter_ref(
    parameter: Any,
    openapi: OpenAPI,
    reference_cls: type = Reference,
):
    if isinstance(parameter, reference_cls):
        return parse_component_ref(parameter.ref, openapi, "parameters", reference_cls)
    return parameter


def resolve_request_body_ref(
    request_body: Any,
    openapi: OpenAPI,
    reference_cls: type = Reference,
):
    if isinstance(request_body, reference_cls):
        return parse_component_ref(request_body.ref, openapi, "requestBodies", reference_cls)
    return request_body


class JsonBodyGenerator:
    def __init__(
        self,
        openapi: OpenAPI,
        reference_cls: type = Reference,
        datatype_cls: type = DataType,
    ):
        self.openapi = openapi
        self.reference_cls = reference_cls
        self.datatype_cls = datatype_cls
        self.cache: dict[str, Any] = {}
        self.seen: set[str] = set()

    def generate_json(self, src: Reference | Schema | MediaType) -> str:
        obj = self.generate(src)
        if obj is None:
            return "{}"
        return json.dumps(obj, indent=2)

    def generate(self, src: Reference | Schema | MediaType):
        if hasattr(src, "media_type_schema"):
            if src.media_type_schema is None:
                return
            return self.generate(src.media_type_schema)

        if isinstance(src, self.reference_cls):
            ref = src.ref
            if ref in self.cache:
                return self.cache[ref]

            if ref in self.seen:
                return

            ref_schema = parse_schema_ref(ref, self.openapi, self.reference_cls)
            if ref_schema is None:
                return

            self.seen.add(ref)
            refobj = self._any_from_schema(ref_schema)
            self.seen.remove(ref)

            self.cache[ref] = refobj
            return refobj
        return self._any_from_schema(src)

    def _any_from_schema(self, schema: Schema):
        dt = self.datatype_cls
        if schema.type == dt.STRING:
            return schema.default or ""
        elif schema.type == dt.NUMBER or schema.type == dt.INTEGER:
            return schema.default or 0
        elif schema.type == dt.BOOLEAN:
            return schema.default or False
        elif schema.type == dt.ARRAY:
            if schema.items is None:
                return []
            item = self.generate(schema.items)
            if item is None:
                return []
            return [item]
        elif schema.type == dt.OBJECT:
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

    version = spec.get("openapi", "")
    models = _get_openapi_models(version)

    info = APIInfo(**spec.get("info", {}))
    external_docs = (
        ExternalDocs(**spec.get("externalDocs", {})) if "externalDocs" in spec else None
    )

    collection_name = spec_path.stem
    raw_servers = spec.get("servers")
    servers = (
        [server for server in raw_servers if isinstance(server, dict)]
        if isinstance(raw_servers, list)
        else []
    ) or [{"url": ""}]

    main_collection = Collection(
        path=spec_path.parent,
        name=collection_name,
    )
    tag_collections: dict[str, Collection] = {}

    openapi = models.OpenAPI.model_validate(spec)
    security_schemes = (
        openapi.components.securitySchemes if openapi.components else None
    ) or {}

    env_files: list[Path] = []
    for server in servers:
        security_variables = {}
        for scheme_name, scheme in security_schemes.items():
            security_variables.update(security_scheme_to_variables(scheme_name, scheme, models.SecurityScheme))

        variables = {**extract_server_variables(server), **security_variables}
        server_url = str(server.get("url", ""))
        env_filename = generate_unique_env_filename(collection_name, server_url)
        env_file = create_env_file(spec_path.parent, env_filename, variables)
        console.print(
            f"Created environment file {str(env_file)!r} for server {server_url!r}."
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
                        request.auth = security_scheme_to_auth(scheme_name, scheme, models.SecurityScheme)
                        break

            parameters = [
                param
                for param in (
                    resolve_parameter_ref(param, openapi, models.Reference)
                    for param in (operation.parameters or [])
                )
                if param is not None
            ]

            # Add query parameters
            for param in parameters:
                if param.param_in == "query":
                    request.params.append(
                        QueryParam(
                            name=param.name,
                            value="",  # Leave empty as it's just a template
                            enabled=not param.deprecated,
                        )
                    )

            # Add headers
            for param in parameters:
                if param.param_in == "header":
                    request.headers.append(
                        Header(
                            name=param.name,
                            value="",  # Leave empty as it's just a template
                            enabled=not param.deprecated,
                        )
                    )

            # Add request body if present
            request_body = resolve_request_body_ref(
                operation.requestBody, openapi, models.Reference
            )
            if isinstance(request_body, models.RequestBody):
                content = request_body.content or {}
                if "application/json" in content:
                    request.body = RequestBody(
                        content=JsonBodyGenerator(openapi, models.Reference, models.DataType).generate_json(
                            content["application/json"]
                        )
                    )
                elif "application/x-www-form-urlencoded" in content:
                    form_data: list[FormItem] = []
                    body = content["application/x-www-form-urlencoded"]
                    for prop_name, _prop_schema in (
                        isinstance(body.media_type_schema, models.Schema)
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
