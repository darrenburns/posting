from posting.importing.curl import CurlImport


def test_simple_get():
    """Test a simple GET request."""
    curl_command = "curl http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.method == "GET"
    assert curl_import.url == "http://example.com"
    assert curl_import.headers == []
    assert curl_import.data is None


def test_get_with_headers():
    """Test GET request with headers."""
    curl_command = "curl -H 'Accept: application/json' -H 'User-Agent: TestAgent' http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.method == "GET"
    assert curl_import.url == "http://example.com"
    assert curl_import.headers == [
        ("Accept", "application/json"),
        ("User-Agent", "TestAgent"),
    ]
    assert curl_import.data is None


def test_post_with_form_data():
    """Test POST request with form data using -d."""
    curl_command = "curl -X POST -d 'key1=value1&key2=value2' http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.method == "POST"
    assert curl_import.url == "http://example.com"
    assert curl_import.data == "key1=value1&key2=value2"
    assert curl_import.is_form_data is True
    assert curl_import.data_pairs == [("key1", "value1"), ("key2", "value2")]


def test_post_with_json_data():
    """Test POST request with JSON data."""
    curl_command = "curl -X POST -H 'Content-Type: application/json' -d '{\"key\":\"value\"}' http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.method == "POST"
    assert curl_import.url == "http://example.com"
    assert curl_import.data == '{"key":"value"}'
    assert curl_import.is_form_data is False
    assert curl_import.data_pairs == []


def test_post_with_form_option():
    """Test POST request with form data using -F."""
    curl_command = (
        "curl -X POST -F 'field1=value1' -F 'field2=value2' http://example.com"
    )
    curl_import = CurlImport(curl_command)
    assert curl_import.method == "POST"
    assert curl_import.url == "http://example.com"
    assert curl_import.form == [("field1", "value1"), ("field2", "value2")]
    assert curl_import.is_form_data is True


def test_multiple_data_options():
    """Test handling of multiple -d options."""
    curl_command = "curl -d 'key1=value1' -d 'key2=value2' http://example.com"
    curl_import = CurlImport(curl_command)
    # Depending on implementation, data might be concatenated or last one used
    assert (
        curl_import.data == "key1=value1key2=value2"
        or curl_import.data == "key2=value2"
    )
    assert curl_import.is_form_data is True
    assert curl_import.data_pairs in (
        [("key1", "value1"), ("key2", "value2")],
        [("key2", "value2")],
    )


def test_curl_with_user_and_password():
    """Test parsing of user credentials."""
    curl_command = "curl -u username:password http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.user == "username:password"
    assert curl_import.url == "http://example.com"


def test_curl_with_bearer_token():
    """Test parsing of user credentials."""
    curl_command = "curl http://example.com -H 'Authorization: Bearer my-token'"
    curl_import = CurlImport(curl_command)
    assert curl_import.headers == [("Authorization", "Bearer my-token")]
    assert curl_import.url == "http://example.com"


def test_curl_with_insecure():
    """Test parsing of --insecure flag."""
    curl_command = "curl -k http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.insecure is True


def test_curl_with_referer():
    """Test parsing of referer."""
    curl_command = "curl -e 'http://referrer.com' http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.referer == "http://referrer.com"


def test_curl_with_user_agent():
    """Test parsing of user-agent."""
    curl_command = "curl -A 'CustomAgent' http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.user_agent == "CustomAgent"


def test_curl_with_compressed():
    """Test parsing of --compressed flag."""
    curl_command = "curl --compressed http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.compressed is True


def test_curl_with_method_and_data():
    """Test custom method with data."""
    curl_command = "curl -X PUT -d 'key=value' http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.method == "PUT"
    assert curl_import.data == "key=value"
    assert curl_import.is_form_data is True


def test_curl_with_data_raw():
    """Test parsing of --data-raw."""
    curl_command = "curl --data-raw 'raw data' http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.data == "raw data"
    assert curl_import.is_form_data is True


def test_curl_with_data_binary():
    """Test parsing of --data-binary."""
    curl_command = "curl --data-binary 'binary data' http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.data == "binary data"
    assert curl_import.is_form_data is True


def test_curl_with_escaped_newlines():
    """Test parsing of curl command with escaped newlines."""
    curl_command = """curl -X POST \\
    -H 'Content-Type: application/json' \\
    -d '{"key": "value"}' \\
    http://example.com"""
    curl_import = CurlImport(curl_command)
    assert curl_import.method == "POST"
    assert curl_import.url == "http://example.com"
    assert curl_import.data == '{"key": "value"}'
    assert curl_import.headers == [("Content-Type", "application/json")]


def test_curl_with_no_space_in_header():
    """Test headers without space after colon."""
    curl_command = "curl -H 'Authorization:Bearer token' http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.headers == [("Authorization", "Bearer token")]


def test_curl_with_complex_command():
    """Test a complex curl command."""
    curl_command = (
        "curl -X POST -H 'Accept: application/json' -H 'Content-Type: application/json' "
        '-d \'{"name":"test","value":123}\' --compressed --insecure '
        "http://example.com/api/test"
    )
    curl_import = CurlImport(curl_command)
    assert curl_import.method == "POST"
    assert curl_import.url == "http://example.com/api/test"
    assert curl_import.headers == [
        ("Accept", "application/json"),
        ("Content-Type", "application/json"),
    ]
    assert curl_import.data == '{"name":"test","value":123}'
    assert curl_import.compressed is True
    assert curl_import.insecure is True
    assert curl_import.is_form_data is False


def test_curl_with_utf8_characters():
    """Test curl command with UTF-8 characters."""
    curl_command = "curl -d 'param=テスト' http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.data == "param=テスト"
    assert curl_import.data_pairs == [("param", "テスト")]


def test_curl_with_special_characters_in_data():
    """Test data containing special characters."""
    curl_command = "curl -d 'param=hello%20world&key=value%3Dtest' http://example.com"
    curl_import = CurlImport(curl_command)
    assert curl_import.data == "param=hello%20world&key=value%3Dtest"
    assert curl_import.data_pairs == [
        ("param", "hello%20world"),
        ("key", "value%3Dtest"),
    ]


def test_curl_imports_max_time():
    """Test a complex example that includes max-time, which was previously not parsed and
    caused imports to be incorrect (fixed in 2.5.1)."""
    curl_command = """\
curl \
  -X POST \
  -H 'Content-Type: application/json' \
  -H 'Accept: *' \
  -H 'Cache-Control: no-cache' \
  -H 'Accept-Encoding: gzip' \
  -d '{
  "string": "Hello, world!",
  "booleans": [true, false],
  "numbers": [1, 2, 42],
  "null": null
}' \
  -u 'darren:' \
  --max-time 0.2 \
  'https://postman-echo.com/post?key1=value1&another-key=another-value&number=123'
"""
    curl_import = CurlImport(curl_command)
    assert curl_import.method == "POST"
    assert (
        curl_import.url
        == "https://postman-echo.com/post?key1=value1&another-key=another-value&number=123"
    )
    assert curl_import.headers == [
        ("Content-Type", "application/json"),
        ("Accept", "*"),
        ("Cache-Control", "no-cache"),
        ("Accept-Encoding", "gzip"),
    ]
    assert (
        curl_import.data
        == """\
{
  "string": "Hello, world!",
  "booleans": [true, false],
  "numbers": [1, 2, 42],
  "null": null
}"""
    )
    assert curl_import.user == "darren:"
