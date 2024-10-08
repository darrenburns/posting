import sys
import httpx

from posting.scripts import Posting


def setup(posting: Posting) -> None:
    posting.set_variable("setup_var", "ADDED IN SETUP")


def on_request(request: httpx.Request, posting: Posting) -> None:
    new_header = "Foo-Bar-Baz!!!!!"
    request.headers["X-Custom-Header"] = new_header
    print(f"Set header to {new_header!r}!")
    posting.notify(
        message="Hello from my_script.py!",
    )
    posting.set_variable("set_in_script", "foo")
    sys.stderr.write("Hello from my_script.py:on_request - i'm an error!")


def on_response(response: httpx.Response, posting: Posting) -> None:
    print(response.status_code)
    print(posting.variables["set_in_script"])  # prints "foo"
    sys.stderr.write("Hello from my_script.py:on_response - i'm an error!")
    sys.stdout.write("Hello from my_script.py!")
