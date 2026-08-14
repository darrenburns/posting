from posting.collection import QueryParam, RequestModel


def test_apply_template_absorbs_url_query_params():
    request = RequestModel(method="GET", url="http://localhost:8000/items/?q=1")
    request.apply_template({})
    assert request.url == "http://localhost:8000/items/"
    assert request.params == [QueryParam(name="q", value="1")]


def test_apply_template_fills_empty_table_value_from_url():
    request = RequestModel(
        method="GET",
        url="http://localhost:8000/items/?q=1",
        params=[QueryParam(name="q", value="")],
    )
    request.apply_template({})
    assert request.url == "http://localhost:8000/items/"
    assert request.params == [QueryParam(name="q", value="1")]


def test_apply_template_resolves_variables_in_url_query():
    request = RequestModel(method="GET", url="$BASE_URL?api_key=$API_KEY")
    request.apply_template(
        {
            "BASE_URL": "https://api.nasa.gov/planetary/apod",
            "API_KEY": "DEMO_KEY",
        }
    )
    assert request.url == "https://api.nasa.gov/planetary/apod"
    assert request.params == [QueryParam(name="api_key", value="DEMO_KEY")]
