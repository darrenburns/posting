import json
import pytest
from textual.widgets import TabbedContent
from posting.__main__ import make_posting
from posting.importing.open_api import import_openapi_spec
from test_snapshots import use_config


@use_config("general.yaml")
@pytest.mark.parametrize("pane", ["path-pane", "query-pane"])
def test_imported_parameters_visible(tmp_path, snap_compare, pane):
    spec = {
        "openapi": "3.1.0",
        "info": {"title": "API", "version": "1"},
        "paths": {
            "/users/{user-id}": {
                "parameters": [
                    {
                        "name": "user-id",
                        "in": "path",
                        "required": True,
                        "schema": {"type": "string"},
                    },
                    {"name": "search", "in": "query", "schema": {"type": "string"}},
                ],
                "get": {
                    "summary": "Find user",
                    "responses": {"200": {"description": "OK"}},
                },
            }
        },
    }
    path = tmp_path / "api.json"
    path.write_text(json.dumps(spec))
    imported = import_openapi_spec(path).requests[0]
    imported.path_params[0].value = "42"
    imported.params[0].value = "Ada"
    app = make_posting(collection=tmp_path)

    async def run_before(pilot):
        await pilot.pause()
        app.screen.load_request_model(imported)
        app.screen.query_one("RequestEditorTabbedContent", TabbedContent).active = pane
        app.screen.collection_browser.border_subtitle = "collection"
        await pilot.pause()
        assert "search=Ada" in app.screen.url_bar.url_input.value
        assert ":user_id" in app.screen.url_bar.url_input.value

    assert snap_compare(app, run_before=run_before, terminal_size=(120, 40))
