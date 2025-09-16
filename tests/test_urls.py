from posting.urls import (
    ensure_protocol,
    extract_path_param_names,
    substitute_path_params,
)


def test_ensure_protocol():
    assert ensure_protocol("example.com") == "http://example.com"
    assert ensure_protocol("https://example.com") == "https://example.com"
    assert ensure_protocol("http://example.com") == "http://example.com"
    assert ensure_protocol("ftp://example.com") == "ftp://example.com"
    assert ensure_protocol("localhost:8000") == "http://localhost:8000"
    assert ensure_protocol("localhost") == "http://localhost"
    assert ensure_protocol("localhost:8000/path") == "http://localhost:8000/path"
    assert ensure_protocol("localhost/path") == "http://localhost/path"
    assert (
        ensure_protocol("localhost:8000/path/to/resource")
        == "http://localhost:8000/path/to/resource"
    )


def test_extract_path_param_names_with_escaping():
    assert extract_path_param_names("/users/:id") == ["id"]
    assert extract_path_param_names("/users/::id") == []
    assert extract_path_param_names("/users/::id/:id") == ["id"]
    assert extract_path_param_names("http://example.com/a/::literal/:name/b") == [
        "name"
    ]


def test_substitute_path_params_with_escaping_simple():
    url = "http://example.com/users/:id"
    out = substitute_path_params(url, {"id": "123"})
    assert out == "http://example.com/users/123"


def test_substitute_path_params_preserves_escaped_literal():
    url = "http://example.com/users/::id"
    out = substitute_path_params(url, {"id": "123"})
    # Escaped token should remain literal ":id" in the path
    assert out == "http://example.com/users/:id"


def test_substitute_path_params_mixed_escaped_and_real():
    url = "http://example.com/users/::id/:id/details"
    out = substitute_path_params(url, {"id": "123"})
    assert out == "http://example.com/users/:id/123/details"
