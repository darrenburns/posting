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


def test_query_variables_keep_reserved_characters_in_one_value():
    request = RequestModel(url='http://example.com/?token=$TOKEN')
    value = 'a&b+c=d#part%20 日本'
    request.apply_template({'TOKEN': value})
    assert request.params == [QueryParam(name='token', value=value)]


def test_variable_containing_entire_url_preserves_repeated_query():
    request = RequestModel(url='$URL')
    request.apply_template({'URL': 'http://example.com/?q=1&q=2&blank='})
    assert request.params == [
        QueryParam(name='q', value='1'),
        QueryParam(name='q', value='2'),
        QueryParam(name='blank', value=''),
    ]


def test_percent_encoded_dollar_is_literal_not_a_variable():
    request = RequestModel(url='http://example.com/?q=%24NOT_A_VARIABLE')
    request.apply_template({})
    assert request.params == [QueryParam(name='q', value='$NOT_A_VARIABLE')]
