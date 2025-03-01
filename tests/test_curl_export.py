import pytest
from posting.collection import (
    RequestModel,
    Header,
    QueryParam,
    Auth,
    FormItem,
    Cookie,
    Options,
    RequestBody,
)


@pytest.fixture
def basic_request():
    """Fixture providing a basic request model for testing."""
    return RequestModel(
        name="Test Request", method="GET", url="https://example.com/api"
    )


def test_simple_get_request():
    """Test a simple GET request with no parameters."""
    request = RequestModel(
        name="Simple GET", method="GET", url="https://example.com/api"
    )

    expected = "curl \\\n  'https://example.com/api'"
    assert request.to_curl() == expected


def test_post_request_with_body():
    """Test a POST request with a JSON body."""
    request = RequestModel(
        name="POST with body",
        method="POST",
        url="https://example.com/api/users",
        body=RequestBody(content='{"name": "John Doe", "email": "john@example.com"}'),
    )

    expected = 'curl \\\n  -X POST \\\n  -d \'{"name": "John Doe", "email": "john@example.com"}\' \\\n  \'https://example.com/api/users\''
    assert request.to_curl() == expected


def test_request_with_headers():
    """Test a request with custom headers."""
    request = RequestModel(
        name="Request with headers",
        method="GET",
        url="https://example.com/api",
        headers=[
            Header(name="Content-Type", value="application/json"),
            Header(name="Authorization", value="Bearer token123"),
            Header(name="Disabled-Header", value="value", enabled=False),
        ],
    )

    expected = "curl \\\n  -H 'Content-Type: application/json' \\\n  -H 'Authorization: Bearer token123' \\\n  'https://example.com/api'"
    assert request.to_curl() == expected


def test_request_with_query_params():
    """Test a request with query parameters."""
    request = RequestModel(
        name="Request with query params",
        method="GET",
        url="https://example.com/api/search",
        params=[
            QueryParam(name="q", value="test query"),
            QueryParam(name="page", value="1"),
            QueryParam(name="disabled", value="true", enabled=False),
        ],
    )

    expected = "curl \\\n  'https://example.com/api/search?q=test+query&page=1'"
    assert request.to_curl() == expected


def test_request_with_existing_query_params():
    """Test a request with existing query parameters in the URL."""
    request = RequestModel(
        name="Request with existing query params",
        method="GET",
        url="https://example.com/api/search?existing=value",
        params=[
            QueryParam(name="q", value="test query"),
            QueryParam(name="page", value="1"),
        ],
    )

    expected = (
        "curl \\\n  'https://example.com/api/search?existing=value&q=test+query&page=1'"
    )
    assert request.to_curl() == expected


def test_request_with_url_fragment():
    """Test a request with a URL fragment."""
    request = RequestModel(
        name="Request with URL fragment",
        method="GET",
        url="https://example.com/api/docs#section1",
        params=[QueryParam(name="version", value="1.0")],
    )

    expected = "curl \\\n  'https://example.com/api/docs?version=1.0#section1'"
    assert request.to_curl() == expected


def test_request_with_basic_auth():
    """Test a request with basic authentication."""
    request = RequestModel(
        name="Request with basic auth",
        method="GET",
        url="https://example.com/api/secure",
        auth=Auth.basic_auth("username", "password"),
    )

    expected = (
        "curl \\\n  -u 'username:password' \\\n  'https://example.com/api/secure'"
    )
    assert request.to_curl() == expected


def test_request_with_digest_auth():
    """Test a request with digest authentication."""
    request = RequestModel(
        name="Request with digest auth",
        method="GET",
        url="https://example.com/api/secure",
        auth=Auth.digest_auth("username", "password"),
    )

    expected = "curl \\\n  --digest -u 'username:password' \\\n  'https://example.com/api/secure'"
    assert request.to_curl() == expected


def test_request_with_cookies():
    """Test a request with cookies."""
    request = RequestModel(
        name="Request with cookies",
        method="GET",
        url="https://example.com/api",
        cookies=[
            Cookie(name="session", value="abc123"),
            Cookie(name="preference", value="dark-mode"),
            Cookie(name="disabled", value="value", enabled=False),
        ],
    )

    expected = "curl \\\n  --cookie 'session=abc123' \\\n  --cookie 'preference=dark-mode' \\\n  'https://example.com/api'"
    assert request.to_curl() == expected


def test_request_with_form_data():
    """Test a request with form data."""
    request = RequestModel(
        name="Request with form data",
        method="POST",
        url="https://example.com/api/form",
        body=RequestBody(
            form_data=[
                FormItem(name="name", value="John Doe"),
                FormItem(name="email", value="john@example.com"),
                FormItem(name="disabled", value="value", enabled=False),
            ]
        ),
    )

    expected = "curl \\\n  -X POST \\\n  -F 'name=John Doe' \\\n  -F 'email=john@example.com' \\\n  'https://example.com/api/form'"
    assert request.to_curl() == expected


def test_request_with_options():
    """Test a request with custom options."""
    request = RequestModel(
        name="Request with options",
        method="GET",
        url="https://example.com/api",
        options=Options(
            follow_redirects=False,
            verify_ssl=False,
            timeout=10.0,
            proxy_url="http://proxy.example.com:8080",
        ),
    )

    expected = "curl \\\n  --no-location \\\n  --insecure \\\n  --max-time 10.0 \\\n  --proxy 'http://proxy.example.com:8080' \\\n  'https://example.com/api'"
    assert request.to_curl() == expected


def test_complex_request():
    """Test a complex request with multiple components."""
    request = RequestModel(
        name="Complex request",
        method="POST",
        url="https://example.com/api/users?existing=value#section",
        headers=[
            Header(name="Content-Type", value="application/json"),
            Header(name="Authorization", value="Bearer token123"),
        ],
        params=[
            QueryParam(name="q", value="test query"),
            QueryParam(name="page", value="1"),
        ],
        body=RequestBody(content='{"name": "John Doe", "email": "john@example.com"}'),
        auth=Auth.basic_auth("username", "password"),
        cookies=[Cookie(name="session", value="abc123")],
        options=Options(verify_ssl=False),
    )

    expected = "curl \\\n  -X POST \\\n  -H 'Content-Type: application/json' \\\n  -H 'Authorization: Bearer token123' \\\n  -d '{\"name\": \"John Doe\", \"email\": \"john@example.com\"}' \\\n  -u 'username:password' \\\n  --cookie 'session=abc123' \\\n  --insecure \\\n  'https://example.com/api/users?existing=value&q=test+query&page=1#section'"
    assert request.to_curl() == expected


def test_special_characters_in_url_and_params():
    """Test handling of special characters in URL and parameters."""
    request = RequestModel(
        name="Special characters",
        method="GET",
        url="https://example.com/api/search",
        params=[
            QueryParam(name="q", value="special chars: !@#$%^&*()"),
            QueryParam(name="filter", value="name=John Doe"),
        ],
    )

    expected = "curl \\\n  'https://example.com/api/search?q=special+chars%3A+%21%40%23%24%25%5E%26%2A%28%29&filter=name%3DJohn+Doe'"
    assert request.to_curl() == expected
