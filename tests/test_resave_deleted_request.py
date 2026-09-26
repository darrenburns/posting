import pytest
from posting.__main__ import make_posting
from posting.collection import RequestModel
from posting.widgets.collection.browser import CollectionTree
from test_snapshots import use_config


@use_config("general.yaml")
@pytest.mark.parametrize("nested", [False, True])
def test_resave_deleted_open_request(tmp_path, snap_compare, nested):
    directory = tmp_path / "folder" if nested else tmp_path
    directory.mkdir(exist_ok=True)
    request_path = directory / "resaved.posting.yaml"
    RequestModel(
        name="Resaved request", url="https://example.com", path=request_path
    ).save_to_disk(request_path)
    app = make_posting(collection=tmp_path)

    async def run_before(pilot):
        await pilot.pause()
        tree = app.screen.query_one(CollectionTree)
        parent = tree.root.children[0] if nested else tree.root
        tree.move_cursor(parent.children[0])
        tree.focus()
        await pilot.press("enter")
        await pilot.pause()
        assert tree.currently_open is not None
        tree.action_delete_request()
        assert not request_path.exists()
        assert not parent.children
        await app.screen.action_save_request()
        await pilot.pause()
        assert request_path.exists()
        assert len(parent.children) == 1
        assert tree.currently_open is parent.children[0]
        await app.screen.action_save_request()
        assert len(parent.children) == 1
        app.clear_notifications()
        tree.focus()
        app.screen.collection_browser.border_subtitle = "collection"

    assert snap_compare(app, run_before=run_before, terminal_size=(120, 40))
