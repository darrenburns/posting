def on_request(request):
    request.headers["X-Custom-Header"] = "Custom-Values edited"


def on_response():
    pass
