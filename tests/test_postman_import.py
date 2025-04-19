from pathlib import Path
from posting.collection import Collection, Options, QueryParam, RequestModel, Scripts
from posting.importing.postman import Variable, import_postman_spec


def test_import_postman_spec():
    # Write mock Postman spec to a temporary file
    collection, postman_collection = import_postman_spec(
        spec_path="tests/sample-importable-collections/test-postman-collection.json",
        output_path="foo/bar/baz",
    )

    assert collection == Collection(
        path=Path("foo/bar/baz"),
        name="Test API",
        requests=[],
        children=[
            Collection(
                path=Path("foo/bar/baz/Users"),
                name="Users",
                requests=[
                    RequestModel(
                        name="Get Users",
                        description="",
                        method="GET",
                        url="$HOST/api/users",
                        path=Path("foo/bar/baz/Users/GetUsers.posting.yaml"),
                        body=None,
                        auth=None,
                        headers=[],
                        params=[
                            QueryParam(
                                name="email", value="example@gmail.com", enabled=True
                            ),
                            QueryParam(
                                name="relations",
                                value="organization,impersonating_user",
                                enabled=True,
                            ),
                        ],
                        cookies=[],
                        posting_version="2.6.0",
                        scripts=Scripts(setup=None, on_request=None, on_response=None),
                        options=Options(
                            follow_redirects=True,
                            verify_ssl=True,
                            attach_cookies=True,
                            proxy_url="",
                            timeout=5.0,
                        ),
                    )
                ],
                children=[
                    Collection(
                        path=Path("foo/bar/baz/Users/User Details"),
                        name="User Details",
                        requests=[
                            RequestModel(
                                name="Get User",
                                description="",
                                method="GET",
                                url="$BASE_URL/users/{id}",
                                path=Path(
                                    "foo/bar/baz/Users/User Details/GetUser.posting.yaml"
                                ),
                                body=None,
                                auth=None,
                                headers=[],
                                params=[],
                                cookies=[],
                                posting_version="2.6.0",
                                scripts=Scripts(
                                    setup=None, on_request=None, on_response=None
                                ),
                                options=Options(
                                    follow_redirects=True,
                                    verify_ssl=True,
                                    attach_cookies=True,
                                    proxy_url="",
                                    timeout=5.0,
                                ),
                            )
                        ],
                        children=[],
                        readme=None,
                    )
                ],
                readme=None,
            )
        ],
        readme="# Test API\n\nA test API\n\nVersion: 2.0.0",
    )

    assert postman_collection.variable == [
        Variable(key="baseUrl", value="https://api.example.com")
    ]
