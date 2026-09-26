from posting.urls import (
    ensure_protocol,
    extract_path_param_names,
    extract_query_pairs,
    merge_url_query_into_params,
    set_query_pairs,
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


def test_extract_query_pairs_keeps_blanks_and_order():
    assert extract_query_pairs("http://localhost:8000/items/?q=1&debug") == [
        ("q", "1"),
        ("debug", ""),
    ]
    assert extract_query_pairs("http://localhost:8000/items/?q=1&q=2") == [
        ("q", "1"),
        ("q", "2"),
    ]
    assert extract_query_pairs("http://localhost:8000/items/?q=") == [("q", "")]
    assert extract_query_pairs("http://localhost:8000/items/") == []


def test_set_query_pairs_replaces_query_and_keeps_path():
    url = "http://localhost:8000/items/?old=1#frag"
    assert set_query_pairs(url, [("q", "1")]) == "http://localhost:8000/items/?q=1#frag"
    assert set_query_pairs(url, []) == "http://localhost:8000/items/#frag"


def test_merge_url_query_fills_empty_table_value():
    url, params = merge_url_query_into_params(
        "http://localhost:8000/items/?q=1",
        [("q", "", True)],
    )
    assert url == "http://localhost:8000/items/"
    assert params == [("q", "1", True)]


def test_merge_url_query_keeps_nonempty_table_value():
    url, params = merge_url_query_into_params(
        "http://localhost:8000/items/?q=1",
        [("q", "from-table", True)],
    )
    assert url == "http://localhost:8000/items/"
    assert params == [("q", "from-table", True)]


def test_merge_url_query_appends_table_only_params():
    url, params = merge_url_query_into_params(
        "http://localhost:8000/items/?q=1",
        [("page", "2", True)],
    )
    assert url == "http://localhost:8000/items/"
    assert params == [("q", "1", True), ("page", "2", True)]


def test_set_query_pairs_preserves_variable_syntax():
    url = "http://localhost:8000/items/"
    assert (
        set_query_pairs(url, [("api_key", "$API_KEY")])
        == "http://localhost:8000/items/?api_key=$API_KEY"
    )
    assert (
        set_query_pairs(url, [("api_key", "${API_KEY}")])
        == "http://localhost:8000/items/?api_key=${API_KEY}"
    )


def test_merge_query_matches_duplicate_rows_by_occurrence():
    _, params = merge_url_query_into_params(
        'http://example.com/?q=first&q=second',
        [('q', '', True), ('q', '', False), ('q', 'third', True)],
    )
    assert params == [('q', 'first', True), ('q', 'second', False), ('q', 'third', True)]
