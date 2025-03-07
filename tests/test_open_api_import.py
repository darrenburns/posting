import json
from pathlib import Path

from posting.importing.open_api import import_openapi_spec


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
