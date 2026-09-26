import httpx
import pytest
from posting.collection import FormItem, Header, RequestBody, RequestModel


def test_multipart_files_and_repeated_fields(tmp_path):
    (tmp_path / "hello.txt").write_text("Hello upload!")
    request = RequestModel(
        method="POST",
        url="https://example.com",
        headers=[
            Header(name="Content-Type", value="application/x-www-form-urlencoded"),
            Header(name="X-Test", value="one"),
            Header(name="X-Test", value="two"),
        ],
        body=RequestBody(
            form_data=[
                FormItem(name="attachment", value="@hello.txt"),
                FormItem(name="tag", value="a"),
                FormItem(name="tag", value="b"),
                FormItem(name="literal", value="@@hello"),
                FormItem(name="disabled", value="@missing", enabled=False),
            ]
        ),
    )
    outgoing = request.to_httpx(httpx.AsyncClient(), base_directory=tmp_path)
    assert outgoing.headers["Content-Type"].startswith("multipart/form-data; boundary=")
    assert outgoing.headers.get_list("X-Test") == ["one", "two"]
    body = outgoing.read()
    assert b'filename="hello.txt"' in body
    assert b"Hello upload!" in body
    assert body.count(b'name="tag"') == 2
    assert b"@hello" in body
    assert b"disabled" not in body


def test_missing_file_is_clear_error(tmp_path):
    body = RequestBody(form_data=[FormItem(name="upload", value="@missing")])
    with pytest.raises(ValueError, match="Cannot read upload file"):
        body.to_httpx_args(tmp_path)


def test_raw_content_type_preserved():
    request = RequestModel(
        method="POST",
        url="https://example.com",
        headers=[Header(name="Content-Type", value="application/custom+json")],
        body=RequestBody(content='{"hello":1}'),
    )
    assert (
        request.to_httpx(httpx.AsyncClient()).headers["Content-Type"]
        == "application/custom+json"
    )


def test_literal_at_sign_is_urlencoded():
    body = RequestBody(form_data=[FormItem(name="text", value="@@literal")])
    assert body.to_httpx_args() == {"data": {"text": ["@literal"]}}
