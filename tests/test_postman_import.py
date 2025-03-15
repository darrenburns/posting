import json
import pytest
from posting.importing.postman import import_postman_spec


@pytest.fixture
def mock_postman_spec():
    return {
        "info": {
            "name": "Test API",
            "description": "A test API",
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
        },
        "variable": [{"key": "baseUrl", "value": "https://api.example.com"}],
        "item": [
            {
                "name": "Users",
                "item": [
                    {
                        "name": "Get Users",
                        "request": {
                            "method": "GET",
                            "url": {
                                "raw": "{{host}}/api/users?email=example@gmail.com&relations=organization,impersonating_user",
                                "host": ["{{host}}"],
                                "path": ["api", "users"],
                                "query": [
                                    {"key": "email", "value": "example@gmail.com"},
                                    {
                                        "key": "relations",
                                        "value": "organization,impersonating_user",
                                    },
                                ],
                            },
                        },
                    },
                    {
                        "name": "User Details",
                        "item": [
                            {
                                "name": "Get User",
                                "request": {
                                    "method": "GET",
                                    "url": {
                                        "raw": "{{baseUrl}}/users/{id}",
                                        "host": ["{{baseUrl}}"],
                                        "path": ["users", "{id}"],
                                    },
                                },
                            }
                        ],
                    },
                ],
            }
        ],
    }


def test_import_postman_spec(tmp_path, mock_postman_spec):
    # Write mock Postman spec to a temporary file
    spec_path = tmp_path / "test_collection.json"
    with open(spec_path, "w") as f:
        json.dump(mock_postman_spec, f)

    # Import the collection
    output_dir = tmp_path / "output"
    collection = import_postman_spec(spec_path, output_path=output_dir)

    # Verify the collection metadata
    assert collection.name == "Test API"
    assert collection.path == output_dir

    # Verify the directory structure was created
    assert output_dir.exists()
    assert output_dir.is_dir()
    assert (output_dir / "Users").exists()
    assert (output_dir / "Users").is_dir()
    assert (output_dir / "Users" / "User Details").exists()
    assert (output_dir / "Users" / "User Details").is_dir()

    # Verify the collection structure
    assert len(collection.children) == 1  # Users folder
    users_collection = collection.children[0]
    assert users_collection.name == "Users"
    assert len(users_collection.requests) == 1  # Get Users request
    assert len(users_collection.children) == 1  # User Details folder

    # Check the "Get Users" request
    get_users = users_collection.requests[0]
    assert get_users.name == "Get Users"
    assert get_users.method == "GET"
    assert (
        get_users.url
        == "$HOST/api/users?email=example@gmail.com&relations=organization,impersonating_user"
    )
    assert len(get_users.params) == 2

    # Check params
    email_param = next(p for p in get_users.params if p.name == "email")
    assert email_param.value == "example@gmail.com"
    relations_param = next(p for p in get_users.params if p.name == "relations")
    assert relations_param.value == "organization,impersonating_user"

    # Check the "User Details" subfolder
    user_details = users_collection.children[0]
    assert user_details.name == "User Details"
    assert len(user_details.requests) == 1

    # Check the "Get User" request
    get_user = user_details.requests[0]
    assert get_user.name == "Get User"
    assert get_user.method == "GET"
    assert get_user.url == "$BASE_URL/users/{id}"

    # Verify request files were created
    users_request_file = output_dir / "Users" / "GetUsers.posting.yaml"
    assert users_request_file.exists()
    assert users_request_file.is_file()

    user_details_request_file = (
        output_dir / "Users" / "User Details" / "GetUser.posting.yaml"
    )
    assert user_details_request_file.exists()
    assert user_details_request_file.is_file()

    # Verify file content exists
    assert users_request_file.stat().st_size > 0
    assert user_details_request_file.stat().st_size > 0

    # Verify environment file
    env_file = output_dir / "Test API.env"
    assert env_file.exists()
    assert env_file.is_file()
    env_content = env_file.read_text()
    assert "BASE_URL=https://api.example.com" in env_content
