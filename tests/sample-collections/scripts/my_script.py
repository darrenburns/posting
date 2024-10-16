import sys
import httpx

from posting import Auth, Header, RequestModel, Posting


def setup(posting: Posting) -> None:
    print("Hello from my_script.py:setup!")
    sys.stderr.write("error from setup!\n")
    posting.set_variable("setup_var", "ADDED IN SETUP")


def on_request(request: RequestModel, posting: Posting) -> None:
    new_header = "Foo-Bar-Baz!!!!!"
    header = Header(name="X-Custom-Header", value=new_header)
    request.headers.append(header)
    print(f"Set header:\n{header}!")
    # request.body.content = "asdf"
    request.auth = Auth.basic_auth("username", "password")
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
