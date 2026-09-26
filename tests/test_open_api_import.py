import json
from pathlib import Path

import pytest

from posting.importing.open_api import import_openapi_spec, _get_openapi_models


def test_import(tmp_path: Path):
    """Test importing security schemes."""
    spec = {
        "openapi": "3.1.0",
        "info": {"title": "Test", "version": "1.0", "description": "Test"},
        "paths": {
            "/": {
                "get": {
                    "parameters": [
                        {
                            "name": "page",
                            "in": "query",
                        },
                        {
                            "name": "account_id",
                            "in": "header",
                            "deprecated": True,
                        },
                    ],
                    "responses": {"200": {"description": "OK"}},
                    "security": [
                        {"bearerAuth": []},
                    ],
                }
            }
        },
        "components": {
            "securitySchemes": {
                "bearerAuth": {
                    "type": "http",
                    "scheme": "bearer",
                },
            },
        },
    }
    spec_path = tmp_path / "spec.json"
    spec_path.write_text(json.dumps(spec))
    collection = import_openapi_spec(spec_path)

    assert len(collection.requests) == 1

    request = collection.requests[0]
    assert request.url == "${BASE_URL}/"
    assert request.method == "GET"

    assert len(request.params) == 1
    param = request.params[0]
    assert param.name == "page"
    assert param.value == ""
    assert param.enabled

    assert len(request.headers) == 1
    header = request.headers[0]
    assert header.name == "account_id"
    assert header.value == ""
    assert not header.enabled

    assert request.auth is not None
    assert request.auth.type == "bearer_token"
    assert request.auth.bearer_token is not None
    assert request.auth.bearer_token.token == "${BEARERAUTH_BEARER_TOKEN}"


def test_import_openapi_3_0(tmp_path: Path):
    """Test importing an OpenAPI 3.0.x spec with security, $ref, and tags."""
    spec = {
        "openapi": "3.0.3",
        "info": {"title": "Test 3.0", "version": "1.0", "description": "Test 3.0 spec"},
        "paths": {
            "/pets": {
                "get": {
                    "summary": "List pets",
                    "tags": ["Pets"],
                    "parameters": [
                        {
                            "name": "limit",
                            "in": "query",
                            "schema": {"type": "integer"},
                        },
                    ],
                    "responses": {"200": {"description": "OK"}},
                    "security": [{"basicAuth": []}],
                },
                "post": {
                    "summary": "Create pet",
                    "tags": ["Pets"],
                    "requestBody": {
                        "required": True,
                        "content": {
                            "application/json": {
                                "schema": {"$ref": "#/components/schemas/Pet"},
                            }
                        },
                    },
                    "responses": {"201": {"description": "Created"}},
                },
            },
            "/health": {
                "get": {
                    "summary": "Health check",
                    "responses": {"200": {"description": "OK"}},
                },
            },
        },
        "components": {
            "schemas": {
                "Pet": {
                    "type": "object",
                    "properties": {
                        "name": {"type": "string"},
                        "tag": {"type": "string"},
                    },
                },
            },
            "securitySchemes": {
                "basicAuth": {
                    "type": "http",
                    "scheme": "basic",
                },
            },
        },
    }
    spec_path = tmp_path / "spec30.json"
    spec_path.write_text(json.dumps(spec))
    collection = import_openapi_spec(spec_path)

    # /health has no tag, so it lands on the main collection
    assert len(collection.requests) == 1
    assert collection.requests[0].method == "GET"
    assert collection.requests[0].url == "${BASE_URL}/health"

    # /pets operations are grouped under the "Pets" tag collection
    assert len(collection.children) == 1
    pets_collection = collection.children[0]
    assert pets_collection.name == "Pets"
    assert len(pets_collection.requests) == 2
    requests_by_method = {r.method: r for r in pets_collection.requests}

    get_request = requests_by_method["GET"]
    assert get_request.url == "${BASE_URL}/pets"
    assert len(get_request.params) == 1
    assert get_request.params[0].name == "limit"

    # Security: basic auth on GET /pets
    assert get_request.auth is not None
    assert get_request.auth.type == "basic"
    assert get_request.auth.basic is not None
    assert get_request.auth.basic.username == "${BASICAUTH_USERNAME}"
    assert get_request.auth.basic.password == "${BASICAUTH_PASSWORD}"

    # POST /pets with $ref body
    post_request = requests_by_method["POST"]
    assert post_request.body is not None
    body = json.loads(post_request.body.content)
    assert "name" in body
    assert "tag" in body


def test_import_openapi_3_0_without_components_and_with_server_url(tmp_path: Path):
    """Test importing a 3.0 spec without components still creates requests and env vars."""
    spec = {
        "openapi": "3.0.3",
        "info": {"title": "No Components", "version": "1.0"},
        "servers": [{"url": "https://api.example.com", "description": "Production"}],
        "paths": {
            "/health": {
                "get": {
                    "summary": "Health check",
                    "responses": {"200": {"description": "OK"}},
                }
            }
        },
    }
    spec_path = tmp_path / "spec30_no_components.json"
    spec_path.write_text(json.dumps(spec))

    collection = import_openapi_spec(spec_path)

    assert len(collection.requests) == 1
    assert collection.requests[0].url == "${BASE_URL}/health"

    env_files = list(tmp_path.glob("*.env"))
    assert len(env_files) == 1
    assert "BASE_URL=https://api.example.com" in env_files[0].read_text()


def test_import_openapi_3_0_resolves_parameter_refs(tmp_path: Path):
    """Test importing referenced query and header parameters from components."""
    spec = {
        "openapi": "3.0.3",
        "info": {"title": "Parameter refs", "version": "1.0"},
        "paths": {
            "/pets": {
                "get": {
                    "parameters": [
                        {"$ref": "#/components/parameters/Limit"},
                        {"$ref": "#/components/parameters/TraceId"},
                    ],
                    "responses": {"200": {"description": "OK"}},
                }
            }
        },
        "components": {
            "parameters": {
                "Limit": {
                    "name": "limit",
                    "in": "query",
                    "schema": {"type": "integer"},
                },
                "TraceId": {
                    "name": "X-Trace-Id",
                    "in": "header",
                    "schema": {"type": "string"},
                },
            }
        },
    }
    spec_path = tmp_path / "spec30_parameter_refs.json"
    spec_path.write_text(json.dumps(spec))

    collection = import_openapi_spec(spec_path)

    assert len(collection.requests) == 1
    request = collection.requests[0]
    assert [param.name for param in request.params] == ["limit"]
    assert [header.name for header in request.headers] == ["X-Trace-Id"]


def test_import_openapi_3_0_resolves_request_body_refs(tmp_path: Path):
    """Test importing a referenced request body that itself points at a schema."""
    spec = {
        "openapi": "3.0.3",
        "info": {"title": "Request body refs", "version": "1.0"},
        "paths": {
            "/pets": {
                "post": {
                    "requestBody": {"$ref": "#/components/requestBodies/PetBody"},
                    "responses": {"201": {"description": "Created"}},
                }
            }
        },
        "components": {
            "schemas": {
                "Pet": {
                    "type": "object",
                    "properties": {
                        "name": {"type": "string"},
                        "age": {"type": "integer"},
                    },
                }
            },
            "requestBodies": {
                "PetBody": {
                    "required": True,
                    "content": {
                        "application/json": {
                            "schema": {"$ref": "#/components/schemas/Pet"},
                        }
                    },
                }
            },
        },
    }
    spec_path = tmp_path / "spec30_request_body_refs.json"
    spec_path.write_text(json.dumps(spec))

    collection = import_openapi_spec(spec_path)

    assert len(collection.requests) == 1
    request = collection.requests[0]
    assert request.body is not None
    assert json.loads(request.body.content) == {"name": "", "age": 0}


def test_import_unsupported_version(tmp_path: Path):
    """Test that an unsupported OpenAPI version raises ValueError."""
    spec = {
        "openapi": "2.0",
        "info": {"title": "Test", "version": "1.0"},
        "paths": {},
    }
    spec_path = tmp_path / "spec_v2.json"
    spec_path.write_text(json.dumps(spec))
    with pytest.raises(ValueError, match="Unsupported OpenAPI version"):
        import_openapi_spec(spec_path)


def test_get_openapi_models_30():
    """Test that 3.0 version returns v3_0 models."""
    models = _get_openapi_models("3.0.3")
    OpenAPI = models[0]
    assert "v3_0" in OpenAPI.__module__


def test_get_openapi_models_31():
    """Test that 3.1 version returns default (3.1) models."""
    models = _get_openapi_models("3.1.0")
    OpenAPI = models[0]
    assert "v3_0" not in OpenAPI.__module__


def test_import_inline_path_params(tmp_path: Path):
    """Inline path parameters should be converted to :param style and added to path_params."""
    spec = {
        "openapi": "3.1.0",
        "info": {"title": "Test", "version": "1.0.0"},
        "paths": {
            "/users/{user_id}/posts/{post_id}": {
                "get": {
                    "summary": "Get post",
                    "parameters": [
                        {
                            "name": "user_id",
                            "in": "path",
                            "required": True,
                            "schema": {"type": "string"},
                        },
                        {
                            "name": "post_id",
                            "in": "path",
                            "required": True,
                            "schema": {"type": "integer"},
                        },
                        {
                            "name": "format",
                            "in": "query",
                            "schema": {"type": "string"},
                        },
                    ],
                    "responses": {"200": {"description": "OK"}},
                }
            }
        },
    }
    spec_path = tmp_path / "spec.json"
    spec_path.write_text(json.dumps(spec))
    collection = import_openapi_spec(spec_path)

    assert len(collection.requests) == 1
    request = collection.requests[0]

    assert request.url == "${BASE_URL}/users/:user_id/posts/:post_id"

    assert len(request.path_params) == 2
    param_names = [p.name for p in request.path_params]
    assert "user_id" in param_names
    assert "post_id" in param_names
    for p in request.path_params:
        assert p.value == ""

    assert len(request.params) == 1
    assert request.params[0].name == "format"


def test_import_ref_path_params(tmp_path: Path):
    """$ref path parameters should be resolved and treated identically to inline path params."""
    spec = {
        "openapi": "3.1.0",
        "info": {"title": "example API", "version": "1.0.0"},
        "paths": {
            "/example/{id}": {
                "get": {
                    "summary": "example GET endpoint",
                    "parameters": [
                        {"$ref": "#/components/parameters/IdParam"}
                    ],
                    "responses": {"200": {"description": "successful"}},
                }
            }
        },
        "components": {
            "parameters": {
                "IdParam": {
                    "name": "id",
                    "in": "path",
                    "required": True,
                    "description": "id example",
                    "schema": {"type": "string"},
                }
            }
        },
    }
    spec_path = tmp_path / "spec.json"
    spec_path.write_text(json.dumps(spec))
    collection = import_openapi_spec(spec_path)

    assert len(collection.requests) == 1
    request = collection.requests[0]

    assert request.url == "${BASE_URL}/example/:id"
    assert len(request.path_params) == 1
    assert request.path_params[0].name == "id"
    assert request.path_params[0].value == ""


def test_import_path_params_openapi_30(tmp_path: Path):
    """Path parameters should work identically for OpenAPI 3.0.x specs."""
    spec = {
        "openapi": "3.0.3",
        "info": {"title": "Test 3.0", "version": "1.0"},
        "paths": {
            "/items/{item_id}": {
                "get": {
                    "summary": "Get item",
                    "parameters": [
                        {
                            "name": "item_id",
                            "in": "path",
                            "required": True,
                            "schema": {"type": "integer"},
                        },
                        {
                            "name": "verbose",
                            "in": "query",
                            "schema": {"type": "boolean"},
                        },
                    ],
                    "responses": {"200": {"description": "OK"}},
                }
            }
        },
    }
    spec_path = tmp_path / "spec30.json"
    spec_path.write_text(json.dumps(spec))
    collection = import_openapi_spec(spec_path)

    assert len(collection.requests) == 1
    request = collection.requests[0]

    assert request.url == "${BASE_URL}/items/:item_id"
    assert len(request.path_params) == 1
    assert request.path_params[0].name == "item_id"
    assert request.path_params[0].value == ""
    assert len(request.params) == 1
    assert request.params[0].name == "verbose"


def test_import_path_item_level_params(tmp_path: Path):
    """Path-item level parameters should be merged with operation parameters.

    OpenAPI allows parameters at the path item level (shared across all
    operations on that path) as well as at the operation level. Operation-level
    parameters override path-item parameters with the same (name, in) pair.
    """
    spec = {
        "openapi": "3.1.0",
        "info": {"title": "Test", "version": "1.0.0"},
        "paths": {
            "/orgs/{org_id}/users/{user_id}": {
                "parameters": [
                    {
                        "name": "org_id",
                        "in": "path",
                        "required": True,
                        "schema": {"type": "string"},
                    },
                ],
                "get": {
                    "summary": "Get user",
                    "parameters": [
                        {
                            "name": "user_id",
                            "in": "path",
                            "required": True,
                            "schema": {"type": "string"},
                        },
                        {
                            "name": "verbose",
                            "in": "query",
                            "schema": {"type": "boolean"},
                        },
                    ],
                    "responses": {"200": {"description": "OK"}},
                },
            }
        },
    }
    spec_path = tmp_path / "spec.json"
    spec_path.write_text(json.dumps(spec))
    collection = import_openapi_spec(spec_path)

    assert len(collection.requests) == 1
    request = collection.requests[0]

    assert request.url == "${BASE_URL}/orgs/:org_id/users/:user_id"

    assert len(request.path_params) == 2
    param_names = [p.name for p in request.path_params]
    assert "org_id" in param_names
    assert "user_id" in param_names
    for p in request.path_params:
        assert p.value == ""

    assert len(request.params) == 1
    assert request.params[0].name == "verbose"


def test_import_path_item_params_operation_overrides(tmp_path: Path):
    """Operation-level parameters override path-item parameters with the same (name, in)."""
    spec = {
        "openapi": "3.1.0",
        "info": {"title": "Test", "version": "1.0.0"},
        "paths": {
            "/items/{item_id}": {
                "parameters": [
                    {
                        "name": "item_id",
                        "in": "path",
                        "required": True,
                        "schema": {"type": "string"},
                        "description": "path-level description",
                    },
                ],
                "get": {
                    "summary": "Get item",
                    "parameters": [
                        {
                            "name": "item_id",
                            "in": "path",
                            "required": True,
                            "schema": {"type": "integer"},
                            "description": "operation-level description",
                        },
                    ],
                    "responses": {"200": {"description": "OK"}},
                },
            }
        },
    }
    spec_path = tmp_path / "spec.json"
    spec_path.write_text(json.dumps(spec))
    collection = import_openapi_spec(spec_path)

    assert len(collection.requests) == 1
    request = collection.requests[0]

    assert request.url == "${BASE_URL}/items/:item_id"
    assert len(request.path_params) == 1
    assert request.path_params[0].name == "item_id"


@pytest.mark.parametrize("version", ["3.0.3", "3.1.0"])
def test_import_non_identifier_path_names_and_query_send(tmp_path, version):
    import httpx
    names = ["user-id", "user_id", "1st", "a.b"]
    path = "/users/" + "/".join("{" + name + "}" for name in names)
    spec = {
        "openapi": version,
        "info": {"title": "Parameters", "version": "1.0"},
        "paths": {path: {
            "parameters": [{"name": name, "in": "path", "required": True, "schema": {"type": "string"}} for name in names] + [{"$ref": "#/components/parameters/Search"}],
            "get": {"summary": "Find user", "responses": {"200": {"description": "OK"}}},
        }},
        "components": {"parameters": {"Search": {"name": "search", "in": "query", "schema": {"type": "string"}}}},
    }
    spec_path = tmp_path / "parameters.json"
    spec_path.write_text(json.dumps(spec))
    request = import_openapi_spec(spec_path).requests[0]
    assert len({p.name for p in request.path_params}) == len(names)
    for param in request.path_params:
        param.value = "42"
    request.params[0].value = "a&b"
    request.apply_template({"BASE_URL": "https://example.com"})
    outgoing = request.to_httpx(httpx.AsyncClient())
    assert outgoing.url.path == "/users/42/42/42/42"
    assert outgoing.url.params.multi_items() == [("search", "a&b")]
