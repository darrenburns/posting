import pytest
from pathlib import Path
import json
import tempfile
import shutil
from posting.importing.postman import (
    Variable,
    RequestItem,
    PostmanRequest,
    Url,
    import_postman_spec,
    create_env_file,
    generate_directory_structure,
    create_request_file,
)


@pytest.fixture
def sample_postman_spec():
    return {
        "info": {
            "name": "Test API",
            "description": "A test API",
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
        },
        "variable": [{"key": "base_url", "value": "https://api.example.com"}],
        "item": [
            {
                "name": "Users",
                "item": [
                    {
                        "name": "Get User",
                        "request": {
                            "method": "GET",
                            "url": {
                                "raw": "{{base_url}}/users/1",
                                "query": [{"key": "include", "value": "posts"}],
                            },
                            "header": [{"key": "Accept", "value": "application/json"}],
                        },
                    }
                ],
            }
        ],
    }


@pytest.fixture
def temp_dir():
    temp_dir = tempfile.mkdtemp()
    yield Path(temp_dir)
    shutil.rmtree(temp_dir)


def test_import_postman_spec(sample_postman_spec, temp_dir):
    spec_file = temp_dir / "test_spec.json"
    with open(spec_file, "w") as f:
        json.dump(sample_postman_spec, f)

    collection, env_file, directories = import_postman_spec(spec_file, temp_dir)

    assert collection.name == "Postman Test"
    assert env_file.name == "Test API.env"
    assert len(directories) == 1
    assert directories[0].endswith("Users")


def test_create_env_file(temp_dir):
    variables = [Variable(key="base_url", value="https://api.example.com")]
    env_file = create_env_file(temp_dir, "test.env", variables)

    assert env_file.exists()
    assert env_file.read_text() == "'base_url'='https://api.example.com'"


def test_generate_directory_structure(temp_dir):
    request_obj = PostmanRequest(method="GET", url=Url(raw="{{base_url}}/users/1"))
    request_item = RequestItem(name="Get User", request=request_obj)
    item = RequestItem(name="Users", item=[request_item])
    items = [item]

    directories = generate_directory_structure(items, base_path=temp_dir)

    assert len(directories) == 1
    assert directories[0].endswith("Users")
    assert (temp_dir / "Users" / "Get User.posting.yaml").exists()


def test_create_request_file(temp_dir):
    request_obj = PostmanRequest(
        method="GET",
        url=Url(
            raw="{{base_url}}/users/1", query=[Variable(key="include", value="posts")]
        ),
        header=[Variable(key="Accept", value="application/json")],
    )
    request_data = RequestItem(name="Get User", request=request_obj)

    file_path = temp_dir / "test_request.posting.yaml"
    create_request_file(file_path, request_data)

    assert file_path.exists()
    content = file_path.read_text()
    assert "name: Get User" in content
    assert "method: GET" in content
    assert "url: '{{base_url}}/users/1'" in content
    assert "headers:" in content
    assert "params:" in content
