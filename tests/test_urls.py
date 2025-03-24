from posting.urls import ensure_protocol


def test_ensure_protocol():
    assert ensure_protocol("example.com") == "http://example.com"
    assert ensure_protocol("https://example.com") == "https://example.com"
    assert ensure_protocol("http://example.com") == "http://example.com"
    assert ensure_protocol("ftp://example.com") == "ftp://example.com"
    assert ensure_protocol("localhost:8000") == "http://localhost:8000"
    assert ensure_protocol("localhost") == "http://localhost"
    assert ensure_protocol("localhost:8000/path") == "http://localhost:8000/path"
    assert ensure_protocol("localhost/path") == "http://localhost/path"
    assert ensure_protocol("localhost:8000/path/to/resource") == "http://localhost:8000/path/to/resource"
    