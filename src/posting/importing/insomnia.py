from __future__ import annotations

from pathlib import Path
import re
from typing import Any
from urllib.parse import parse_qsl, urlparse, urlunparse

from rich.console import Console

from posting.collection import (
    APIInfo,
    Auth,
    Collection,
    FormItem,
    Header,
    QueryParam,
    RequestBody,
    RequestModel,
    VALID_HTTP_METHODS,
)
from posting.yaml import Loader, load


def sanitize_variables(string: str) -> str:
    """Converts variable names like base_url, userId, or user-id to uppercase with underscores."""
    underscore_case = re.sub(r"(?<!^)(?=[A-Z])", "_", string)
    underscore_case = re.sub(r"[^A-Za-z0-9_]+", "_", underscore_case)
    return underscore_case.upper()


def sanitize_str(string: str | None) -> str:
    """Replaces Insomnia template tags {{ _.variable }} or {{ variable }} with $VARIABLE."""
    if not string:
        return "" if string is not None else ""

    def replace_match(match: re.Match[str]) -> str:
        value = match.group(1)
        return f"${sanitize_variables(value)}"

    return re.sub(r"\{\{\s*(?:_[\.\s]+)?([\w-]+)\s*\}\}", replace_match, string)


def sanitize_filename(string: str) -> str:
    """Creates a PascalCase filename from a request or folder name."""
    cleaned = "".join(
        word.capitalize()
        for word in re.sub(r"[^A-Za-z0-9\.]+", " ", string).split()
    )
    return cleaned or "Request"


def sanitize_env_name(string: str) -> str:
    """Creates a slug for an environment filename."""
    slug = re.sub(r"[^A-Za-z0-9_-]+", "_", string.strip().lower()).strip("_")
    return slug or "env"


def format_request(name: str, item: dict[str, Any]) -> RequestModel:
    """Format an Insomnia request object into a Posting RequestModel."""
    raw_url = str(item.get("url") or "")

    parsed_url = urlparse(raw_url)
    url_without_query = urlunparse(
        (
            parsed_url.scheme,
            parsed_url.netloc,
            parsed_url.path,
            parsed_url.params,
            "",
            parsed_url.fragment,
        )
    )
    sanitized_url = sanitize_str(url_without_query)

    method_str = str(item.get("method") or "GET").upper()
    method = method_str if method_str in VALID_HTTP_METHODS else "GET"

    description = str(
        item.get("description")
        or (item.get("meta") or {}).get("description")
        or ""
    )

    posting_request = RequestModel(
        name=name,
        method=method,  # type: ignore[arg-type]
        description=description,
        url=sanitized_url,
    )

    if parsed_url.query:
        for q_name, q_val in parse_qsl(parsed_url.query, keep_blank_values=True):
            posting_request.params.append(
                QueryParam(
                    name=sanitize_str(q_name),
                    value=sanitize_str(q_val),
                    enabled=True,
                )
            )

    for param in item.get("parameters") or []:
        if not isinstance(param, dict):
            continue
        p_name = str(param.get("name", ""))
        p_val = "" if param.get("value") is None else str(param.get("value"))
        enabled = not bool(param.get("disabled", False))
        posting_request.params.append(
            QueryParam(
                name=sanitize_str(p_name),
                value=sanitize_str(p_val),
                enabled=enabled,
            )
        )

    for header in item.get("headers") or []:
        if not isinstance(header, dict):
            continue
        if "name" in header:
            h_name = str(header.get("name", ""))
            h_val = "" if header.get("value") is None else str(header.get("value"))
            disabled = bool(header.get("disabled", False))
        else:
            for k, v in header.items():
                h_name = str(k)
                if isinstance(v, dict):
                    h_val = "" if v.get("value") is None else str(v.get("value"))
                    disabled = bool(v.get("disabled", False))
                else:
                    h_val = "" if v is None else str(v)
                    disabled = False
                break
        posting_request.headers.append(
            Header(
                name=sanitize_str(h_name),
                value=sanitize_str(h_val),
                enabled=not disabled,
            )
        )

    body_data = item.get("body")
    if isinstance(body_data, dict):
        mime_type = str(body_data.get("mimeType") or "").lower()
        text = body_data.get("text")
        params = body_data.get("params")

        if "application/x-www-form-urlencoded" in mime_type or (
            text is None and params is not None
        ):
            form_data: list[FormItem] = []
            for p in params or []:
                if isinstance(p, dict):
                    p_name = str(p.get("name", ""))
                    p_val = "" if p.get("value") is None else str(p.get("value"))
                    disabled = bool(p.get("disabled", False))
                    form_data.append(
                        FormItem(
                            name=sanitize_str(p_name),
                            value=sanitize_str(p_val),
                            enabled=not disabled,
                        )
                    )
            posting_request.body = RequestBody(form_data=form_data)
        elif text is not None and "multipart/form-data" not in mime_type:
            posting_request.body = RequestBody(content=sanitize_str(str(text)))
    elif isinstance(body_data, str) and body_data.strip():
        posting_request.body = RequestBody(content=sanitize_str(body_data))

    auth = item.get("authentication")
    if isinstance(auth, dict) and auth and not auth.get("disabled", False):
        auth_type = str(auth.get("type") or "").lower()

        if auth_type in ("bearer", "bearer_token") or "bearer" in auth:
            token = ""
            if auth_type in ("bearer", "bearer_token"):
                token = auth.get("token") or ""
            elif isinstance(auth.get("bearer"), dict):
                token = auth["bearer"].get("token") or ""
            if token:
                posting_request.auth = Auth.bearer_token_auth(
                    token=sanitize_str(str(token))
                )

        elif auth_type == "basic" or "basic" in auth:
            username = ""
            password = ""
            if auth_type == "basic":
                username = auth.get("username") or ""
                password = auth.get("password") or ""
            elif isinstance(auth.get("basic"), dict):
                username = auth["basic"].get("username") or ""
                password = auth["basic"].get("password") or ""
            posting_request.auth = Auth.basic_auth(
                username=sanitize_str(str(username)),
                password=sanitize_str(str(password)),
            )

        elif auth_type == "digest" or "digest" in auth:
            username = ""
            password = ""
            if auth_type == "digest":
                username = auth.get("username") or ""
                password = auth.get("password") or ""
            elif isinstance(auth.get("digest"), dict):
                username = auth["digest"].get("username") or ""
                password = auth["digest"].get("password") or ""
            posting_request.auth = Auth.digest_auth(
                username=sanitize_str(str(username)),
                password=sanitize_str(str(password)),
            )

    return posting_request


def _is_folder(item: dict[str, Any]) -> bool:
    """Determine whether an item is a folder container."""
    if item.get("children") is not None:
        return True
    meta_id = str((item.get("meta") or {}).get("id") or "")
    if meta_id.startswith("fld_"):
        return True
    if "method" not in item and "url" not in item and "children" in item:
        return True
    return False


def process_item(
    item: dict[str, Any], parent_collection: Collection, base_path: Path
) -> None:
    """Process an Insomnia folder or request item recursively."""
    if not isinstance(item, dict):
        return

    name = str(item.get("name") or "")
    item_data = item
    if not name and len(item) == 1:
        key = next(iter(item.keys()))
        val = item[key]
        if isinstance(val, dict):
            name = str(key)
            item_data = dict(val)
            if "name" not in item_data:
                item_data["name"] = name

    if _is_folder(item_data):
        folder_name = name or "Folder"
        child_path = base_path / folder_name
        child_collection = Collection(path=child_path, name=folder_name)
        parent_collection.children.append(child_collection)

        for sub_item in item_data.get("children") or []:
            process_item(sub_item, child_collection, child_path)
    else:
        req_name = name or "Request"
        request = format_request(req_name, item_data)
        file_name = sanitize_filename(req_name)

        candidate_path = parent_collection.path / f"{file_name}.posting.yaml"
        counter = 1
        while any(r.path == candidate_path for r in parent_collection.requests):
            candidate_path = (
                parent_collection.path / f"{file_name}_{counter}.posting.yaml"
            )
            counter += 1

        request.path = candidate_path
        parent_collection.requests.append(request)


def extract_environments(spec: dict[str, Any]) -> dict[str, dict[str, Any]]:
    """Extract base and sub-environments into a mapping of filename -> variable dict."""
    env_files: dict[str, dict[str, Any]] = {}
    env_root = spec.get("environments")

    if not env_root:
        return env_files

    env_blocks = env_root if isinstance(env_root, list) else [env_root]

    for env_block in env_blocks:
        if not isinstance(env_block, dict):
            continue

        base_data = env_block.get("data") or {}
        base_name = str(env_block.get("name") or "Base Environment")
        if base_name.lower() in ("base environment", "base"):
            base_filename = "base.env"
        else:
            base_filename = f"{sanitize_env_name(base_name)}.env"

        if base_data:
            env_files[base_filename] = {
                k: "" if v is None else str(v) for k, v in base_data.items()
            }

        sub_envs = (
            env_block.get("subEnvironments")
            or env_block.get("sub_environments")
            or []
        )
        for sub_env in sub_envs:
            if not isinstance(sub_env, dict):
                continue
            sub_name = str(sub_env.get("name") or "env")
            sub_filename = f"{sanitize_env_name(sub_name)}.env"

            merged_data = dict(base_data)
            sub_data = sub_env.get("data") or {}
            merged_data.update(sub_data)

            if merged_data:
                env_files[sub_filename] = {
                    k: "" if v is None else str(v) for k, v in merged_data.items()
                }

    return env_files


def create_env_file(
    path: Path, env_filename: str, variables: dict[str, Any] | list[Any]
) -> Path:
    """Create a .env file in the specified path with sanitized variable names."""
    env_content: list[str] = []

    if isinstance(variables, dict):
        for key, value in variables.items():
            val = "" if value is None else str(value)
            env_content.append(f"{sanitize_variables(str(key))}={val}")
    elif isinstance(variables, list):
        for item in variables:
            if hasattr(item, "key"):
                val = (
                    ""
                    if getattr(item, "value", None) is None
                    else str(getattr(item, "value"))
                )
                env_content.append(f"{sanitize_variables(str(item.key))}={val}")
            elif isinstance(item, tuple) and len(item) == 2:
                val = "" if item[1] is None else str(item[1])
                env_content.append(f"{sanitize_variables(str(item[0]))}={val}")

    env_file = path / env_filename
    env_file.write_text("\n".join(env_content) + ("\n" if env_content else ""))
    return env_file


def _convert_v4_resources_to_v5_tree(spec_dict: dict[str, Any]) -> dict[str, Any]:
    """Convert flat Insomnia V4 resources list into a V5 hierarchical structure."""
    resources = spec_dict.get("resources") or []
    workspace_name = "Imported Collection"
    workspace_desc = ""
    workspace_id = None

    folders: dict[str, dict[str, Any]] = {}
    requests: dict[str, dict[str, Any]] = {}
    environments_obj: dict[str, Any] = {
        "name": "Base Environment",
        "data": {},
        "subEnvironments": [],
    }

    for res in resources:
        if not isinstance(res, dict):
            continue
        res_type = res.get("_type")
        res_id = res.get("_id", "")

        if res_type == "workspace":
            workspace_name = res.get("name") or workspace_name
            workspace_desc = res.get("description") or workspace_desc
            workspace_id = res_id
        elif res_type == "environment":
            parent_id = res.get("parentId")
            if parent_id and parent_id.startswith("env_"):
                environments_obj["subEnvironments"].append(
                    {
                        "name": res.get("name") or "env",
                        "data": res.get("data") or {},
                    }
                )
            else:
                environments_obj["data"].update(res.get("data") or {})
        elif res_type == "request_group":
            folders[res_id] = {
                "name": res.get("name") or "Folder",
                "parentId": res.get("parentId"),
                "children": [],
            }
        elif res_type == "request":
            requests[res_id] = {
                "name": res.get("name") or "Request",
                "parentId": res.get("parentId"),
                "url": res.get("url"),
                "method": res.get("method"),
                "description": res.get("description"),
                "parameters": res.get("parameters"),
                "headers": res.get("headers"),
                "body": res.get("body"),
                "authentication": res.get("authentication"),
            }

    for folder in list(folders.values()):
        p_id = folder.get("parentId")
        if p_id in folders:
            folders[p_id]["children"].append(folder)

    root_items: list[dict[str, Any]] = []
    for req in requests.values():
        p_id = req.get("parentId")
        if p_id in folders:
            folders[p_id]["children"].append(req)
        else:
            root_items.append(req)

    for folder in folders.values():
        p_id = folder.get("parentId")
        if not p_id or p_id == workspace_id or p_id not in folders:
            root_items.append(folder)

    return {
        "name": workspace_name,
        "meta": {"description": workspace_desc},
        "collection": root_items,
        "environments": environments_obj,
    }


def import_insomnia_spec(
    spec_path: str | Path, output_path: str | Path | None = None
) -> tuple[Collection, dict[str, dict[str, Any]]]:
    """Import an Insomnia V5 YAML/JSON collection from a file."""
    console = Console()
    console.print(f"Importing Insomnia spec from {spec_path!r}.")

    spec_path = Path(spec_path)
    with open(spec_path, "r", encoding="utf-8") as file:
        spec_dict = load(file, Loader=Loader)

    if not isinstance(spec_dict, dict):
        raise ValueError("Invalid Insomnia collection: expected a YAML or JSON mapping")

    if "resources" in spec_dict and not (
        spec_dict.get("collection") or spec_dict.get("requests")
    ):
        spec_dict = _convert_v4_resources_to_v5_tree(spec_dict)

    collection_name = str(spec_dict.get("name") or "Imported Collection")
    meta = spec_dict.get("meta") or {}
    description = str(
        meta.get("description") or spec_dict.get("description") or "No description"
    )

    info = APIInfo(
        title=collection_name,
        description=description,
        specSchema="https://schema.insomnia.rest/v5",
        version=str(spec_dict.get("schema_version") or "5.0"),
    )

    base_dir = spec_path.parent
    if output_path is not None:
        base_dir = Path(output_path) if isinstance(output_path, str) else output_path

    base_dir.mkdir(parents=True, exist_ok=True)
    main_collection = Collection(path=base_dir, name=info.title)
    main_collection.readme = main_collection.generate_readme(info)

    items = spec_dict.get("collection") or spec_dict.get("requests") or []
    for item in items:
        process_item(item, main_collection, base_dir)

    env_files = extract_environments(spec_dict)

    return main_collection, env_files
