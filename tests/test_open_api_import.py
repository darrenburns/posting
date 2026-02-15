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
