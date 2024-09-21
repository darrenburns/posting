def on_request(request):
    request.headers["X-Custom-Header"] = "Custom-Values!!!"


def on_response():
    pass
