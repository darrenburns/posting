from pathlib import Path
from click.testing import CliRunner

from posting.__main__ import cli

from posting.collection import (
    Collection,
    FormItem,
    Header,
    Options,
    QueryParam,
    RequestBody,
    RequestModel,
    Scripts,
)
from posting.importing.insomnia import (
    create_env_file,
    format_request,
    import_insomnia_spec,
    sanitize_str,
    sanitize_variables,
)


def test_sanitize_variables():
    assert sanitize_variables("base_url") == "BASE_URL"
    assert sanitize_variables("userId") == "USER_ID"
    assert sanitize_variables("user-id") == "USER_ID"
    assert sanitize_variables("API_TOKEN") == "API_TOKEN"


def test_sanitize_str():
    assert sanitize_str("{{ _.base_url }}/users") == "$BASE_URL/users"
    assert sanitize_str("{{ base_url }}/users") == "$BASE_URL/users"
    assert sanitize_str("{{  _.userId  }}") == "$USER_ID"
    assert sanitize_str("Bearer {{ _.token }}") == "Bearer $TOKEN"
    assert sanitize_str(None) == ""
    # Dynamic expression tags are left unchanged
    dynamic_tag = "{% response 'body', 'req_auth', '$.token' %}"
    assert sanitize_str(dynamic_tag) == dynamic_tag


def test_import_insomnia_spec():
    collection, env_files = import_insomnia_spec(
        spec_path="tests/sample-importable-collections/test-insomnia-collection.yaml",
        output_path="test/output/insomnia",
    )

    expected_base_options = Options(
        follow_redirects=True,
        verify_ssl=True,
        attach_cookies=True,
        proxy_url="",
        timeout=5.0,
    )
    expected_scripts = Scripts(setup=None, on_request=None, on_response=None)

    assert collection.name == "Test API"
    assert collection.path == Path("test/output/insomnia")
    assert collection.readme == "# Test API\n\nA test API collection from Insomnia\n\nVersion: 5.1\n\n"

    # Verify top-level request (Ping)
    assert len(collection.requests) == 1
    ping_req = collection.requests[0]
    assert ping_req.name == "Ping"
    assert ping_req.method == "GET"
    assert ping_req.url == "$BASE_URL/ping"
    assert ping_req.path == Path("test/output/insomnia/Ping.posting.yaml")
    assert len(ping_req.headers) == 1
    assert ping_req.headers[0] == Header(name="X-Ping-Header", value="Pong", enabled=True)

    # Verify Users folder
    assert len(collection.children) == 1
    users_folder = collection.children[0]
    assert users_folder.name == "Users"
    assert users_folder.path == Path("test/output/insomnia/Users")

    # 3 direct requests in Users folder: Get Users, Basic Auth Check, Digest Auth Check
    assert len(users_folder.requests) == 3
    get_users_req = users_folder.requests[0]
    assert get_users_req.name == "Get Users"
    assert get_users_req.method == "GET"
    assert get_users_req.url == "$BASE_URL/users"
    assert get_users_req.path == Path("test/output/insomnia/Users/GetUsers.posting.yaml")
    assert get_users_req.params == [
        QueryParam(name="page", value="1", enabled=True),
        QueryParam(name="limit", value="10", enabled=False),
    ]
    assert get_users_req.headers == [
        Header(name="Accept", value="application/json", enabled=True)
    ]
    assert get_users_req.auth is not None
    assert get_users_req.auth.type == "bearer_token"
    assert get_users_req.auth.bearer_token.token == "$TOKEN"

    basic_auth_req = users_folder.requests[1]
    assert basic_auth_req.name == "Basic Auth Check"
    assert basic_auth_req.auth is not None
    assert basic_auth_req.auth.type == "basic"
    assert basic_auth_req.auth.basic.username == "$USER_ID"
    assert basic_auth_req.auth.basic.password == "secret_password"

    digest_auth_req = users_folder.requests[2]
    assert digest_auth_req.name == "Digest Auth Check"
    assert digest_auth_req.auth is not None
    assert digest_auth_req.auth.type == "digest"
    assert digest_auth_req.auth.digest.username == "digest_user"
    assert digest_auth_req.auth.digest.password == "digest_password"

    # Verify nested subfolder (User Details)
    assert len(users_folder.children) == 1
    details_folder = users_folder.children[0]
    assert details_folder.name == "User Details"
    assert details_folder.path == Path("test/output/insomnia/Users/User Details")

    assert len(details_folder.requests) == 3
    get_user_req = details_folder.requests[0]
    assert get_user_req.name == "Get User"
    assert get_user_req.url == "$BASE_URL/users/$USER_ID"
    assert get_user_req.params == [
        QueryParam(name="active", value="true", enabled=True)
    ]

    create_user_req = details_folder.requests[1]
    assert create_user_req.name == "Create User"
    assert create_user_req.method == "POST"
    assert create_user_req.body is not None
    assert '{\n  "name": "Jane Doe",\n  "email": "$EMAIL"\n}' in (
        create_user_req.body.content or ""
    )

    form_req = details_folder.requests[2]
    assert form_req.name == "Submit Form"
    assert form_req.method == "POST"
    assert form_req.body is not None
    assert form_req.body.form_data == [
        FormItem(name="username", value="$USER_ID", enabled=True),
        FormItem(name="status", value="active", enabled=False),
    ]

    # Verify Environment Files extraction
    assert set(env_files.keys()) == {"base.env", "local.env", "production.env"}
    assert env_files["base.env"] == {
        "base_url": "https://api.example.com",
        "email": "user@example.com",
        "user_id": "user-123",
    }
    # Local environment has base variables + local overrides
    assert env_files["local.env"] == {
        "base_url": "http://localhost:8080",
        "email": "user@example.com",
        "user_id": "user-123",
        "token": "local-token-123",
    }
    # Production environment has base variables + prod overrides
    assert env_files["production.env"] == {
        "base_url": "https://prod.example.com",
        "email": "user@example.com",
        "user_id": "user-123",
        "token": "prod-token-456",
    }


def test_create_env_file(tmp_path: Path):
    env_vars = {
        "base_url": "http://localhost:8080",
        "api_key": "secret123",
        "debug": "true",
    }
    env_file = create_env_file(tmp_path, "local.env", env_vars)

    assert env_file.exists()
    content = env_file.read_text()
    assert "BASE_URL=http://localhost:8080" in content
    assert "API_KEY=secret123" in content
    assert "DEBUG=true" in content


def test_import_insomnia_cli(tmp_path: Path):
    runner = CliRunner()
    output_dir = tmp_path / "imported"
    result = runner.invoke(
        cli,
        [
            "import",
            "--type",
            "insomnia",
            "tests/sample-importable-collections/test-insomnia-collection.yaml",
            "-o",
            str(output_dir),
        ],
    )
    assert result.exit_code == 0
    assert "Successfully imported 'Insomnia' spec" in result.output

    # Check files created on disk
    assert (output_dir / "Ping.posting.yaml").exists()
    assert (output_dir / "Users" / "GetUsers.posting.yaml").exists()
    assert (output_dir / "Users" / "User Details" / "CreateUser.posting.yaml").exists()
    assert (output_dir / "base.env").exists()
    assert (output_dir / "local.env").exists()
    assert (output_dir / "production.env").exists()


def test_import_insomnia_multipart_ignored():
    item = {
        "name": "Upload File",
        "method": "POST",
        "url": "https://example.com/upload",
        "body": {
            "mimeType": "multipart/form-data",
            "params": [
                {"name": "file", "value": "test.txt", "type": "file"},
            ],
        },
    }
    request = format_request("Upload File", item)
    assert request.body is None


def test_import_insomnia_disabled_auth():
    item = {
        "name": "No Auth",
        "method": "GET",
        "url": "https://example.com",
        "authentication": {
            "type": "bearer",
            "token": "token123",
            "disabled": True,
        },
    }
    request = format_request("No Auth", item)
    assert request.auth is None


def test_import_insomnia_legacy_v4(tmp_path: Path):
    v4_spec = {
        "_type": "export",
        "__export_format": 4,
        "resources": [
            {
                "_id": "wrk_1",
                "_type": "workspace",
                "name": "Legacy V4 API",
                "description": "Legacy format description",
            },
            {
                "_id": "env_1",
                "_type": "environment",
                "parentId": "wrk_1",
                "data": {"host": "https://legacy.example.com"},
            },
            {
                "_id": "env_2",
                "_type": "environment",
                "parentId": "env_1",
                "name": "Staging",
                "data": {"host": "https://staging.example.com"},
            },
            {
                "_id": "fld_1",
                "_type": "request_group",
                "parentId": "wrk_1",
                "name": "Items",
            },
            {
                "_id": "req_1",
                "_type": "request",
                "parentId": "fld_1",
                "name": "List Items",
                "method": "GET",
                "url": "{{ _.host }}/items",
            },
        ],
    }

    import yaml

    spec_file = tmp_path / "legacy.yaml"
    spec_file.write_text(yaml.dump(v4_spec))

    collection, env_files = import_insomnia_spec(spec_file, tmp_path / "legacy_out")
    assert collection.name == "Legacy V4 API"
    assert len(collection.children) == 1
    assert collection.children[0].name == "Items"
    assert len(collection.children[0].requests) == 1
    req = collection.children[0].requests[0]
    assert req.name == "List Items"
    assert req.url == "$HOST/items"
    assert "base.env" in env_files
    assert "staging.env" in env_files
    assert env_files["staging.env"]["host"] == "https://staging.example.com"
