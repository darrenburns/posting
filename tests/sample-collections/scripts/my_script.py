import httpx

def on_request(request: httpx.Request) -> None:
    new_header = "Foo-Bar-Baz!!!!!"
    request.headers["X-Custom-Header"] = new_header
    print(f"Set header to {new_header!r}!")


def on_response(response: httpx.Response) -> None:
    print(response.status_code)
