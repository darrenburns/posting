import json
from pathlib import Path
from unittest.mock import patch, mock_open

import pytest
from posting.importing.postman import import_postman_spec, PostmanCollection
from posting.collection import Collection


@pytest.fixture
def sample_postman_spec():
    return {
        "info": {
            "name": "Test API",
            "description": "A test API",
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
        },
        "variable": [{"key": "baseUrl", "value": "https://api.example.com"}],
        "item": [
            {
                "name": "Get Users",
                "request": {
                    "method": "GET",
                    "url": {
                        "raw": "{{baseUrl}}/users",
                        "host": ["{{baseUrl}}"],
                        "path": ["users"],
                    },
                },
            }
        ],
    }


@pytest.fixture
def mock_spec_file(sample_postman_spec):
    return mock_open(read_data=json.dumps(sample_postman_spec))


def test_import_postman_spec(sample_postman_spec, mock_spec_file):
    spec_path = Path("/path/to/spec.json")
    output_path = Path("/path/to/output")

    with patch("builtins.open", mock_spec_file), patch(
        "posting.importing.postman.create_env_file"
    ) as mock_create_env, patch(
        "posting.collection.Collection.generate_readme"
    ) as mock_generate_readme:
        mock_create_env.return_value = output_path / "Test API.env"
        mock_generate_readme.return_value = "# Test API"

        result = import_postman_spec(spec_path, output_path)

        assert isinstance(result, Collection)
        assert result.name == "Test API"
        assert len(result.requests) == 1
        assert result.requests[0].name == "GetUsers"
        assert result.requests[0].method == "GET"
        assert result.requests[0].url == "$BASE_URL/users"

        mock_create_env.assert_called_once_with(
            output_path,
            "Test API.env",
            PostmanCollection(**sample_postman_spec).variable,
        )
        mock_generate_readme.assert_called_once()
