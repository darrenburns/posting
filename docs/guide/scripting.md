## Overview

You can attach simple Python scripts to requests, and have Posting run them before and/or after the request is made. This powerful feature allows you to:

- Perform setup before a request (e.g. setting headers, preparing data)
- Print logs and messages
- Set variables to be used in later requests (e.g. authentication tokens)
- Inspect request and response objects, and manipulate them
- Pretty much anything else you can think of doing with Python!

## Script Types

Posting supports two types of scripts:

1. **Pre-request Scripts**: Runs before the request is sent, but after variables have been substituted.
2. **Post-response Scripts**: Runs after the response is received.

## Writing Scripts

In the context of Posting, a "script" is a regular Python function.

By default, if you specify a path to a Python file, Posting will look for and execute the `on_request` and `on_response` functions at the appropriate times.

- `on_request(request: httpx.Request, posting: Posting) -> None`
- `on_response(response: httpx.Response, posting: Posting) -> None`

You can also specify the name of the function using the format `path/to/script.py:function_to_run`.

Note that you do not need to specify all of the arguments when writing these functions. Posting will only pass the number of arguments that you've specified when it calls your function.

### The `Posting` Object

The `Posting` object provides access to the application context and useful methods:

- `set_variable(name: str, value: str) -> None`: Set a session variable
- `get_variable(name: str) -> str | None`: Get a session variable
- `clear_variable(name: str) -> None`: Clear a specific session variable
- `clear_all_variables() -> None`: Clear all session variables
- `notify(message: str, title: str = "", severity: str = "information", timeout: float | None = None)`: Send a notification to the user

### Example: Pre-request Script

```python
def on_request(request: httpx.Request, posting: Posting) -> None:
    # Set a custom header on the request.
    request.headers["X-Custom-Header"] = "foo"

    # This will be captured and written to the log.
    print("Request is being sent!")

    # Make a notification pop-up in the UI.
    posting.notify("Request is being sent!")
```

### Example: Post-response Script

```python
def on_response(response: httpx.Response, posting: Posting) -> None:
    # Print the status code of the response to the log.
    print(response.status_code)

    # Set a variable to be used in later requests.
    # You can write '$auth_token' in the UI and it will be substituted with
    # the value of the $auth_token variable.
    posting.set_variable("auth_token", response.headers["Authorization"])
```

