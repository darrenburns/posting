"""Tests for collection_browser.open_on_highlight."""

import asyncio

import pytest

from posting.__main__ import make_posting
from posting.collection import RequestModel
from posting.widgets.collection.browser import CollectionTree
from test_snapshots import use_config


@pytest.fixture
def two_requests(tmp_path):
    for name in ("alpha", "beta"):
        path = tmp_path / f"{name}.posting.yaml"
        RequestModel(name=name, url=f"https://{name}.example.com", path=path).save_to_disk(
            path
        )
    return tmp_path


def _tree(app) -> CollectionTree:
    return app.screen.query_one(CollectionTree)


@use_config("general.yaml")
def test_highlight_does_not_open_by_default(two_requests):
    async def check():
        app = make_posting(collection=two_requests)
        async with app.run_test(size=(120, 34)) as pilot:
            await pilot.pause()
            tree = _tree(app)
            tree.focus()
            await pilot.press("j")
            await pilot.pause()
            assert isinstance(tree.cursor_node.data, RequestModel)
            assert tree.currently_open is None
            app.clear_notifications()

    asyncio.run(check())


@use_config("open_on_highlight.yaml")
def test_highlight_opens_when_enabled(two_requests):
    async def check():
        app = make_posting(collection=two_requests)
        async with app.run_test(size=(120, 34)) as pilot:
            await pilot.pause()
            tree = _tree(app)
            tree.focus()
            # Let the app finish its startup mount waves (the Query tab's
            # ParamsTable mounts lazily a couple of frames in).
            await pilot.pause()
            await pilot.pause()
            await pilot.press("j")
            await pilot.pause()
            assert isinstance(tree.cursor_node.data, RequestModel)
            assert tree.currently_open is tree.cursor_node
            # Stepping to the next request re-opens; the editor follows the cursor.
            await pilot.press("j")
            await pilot.pause()
            assert tree.currently_open is tree.cursor_node
            # Stepping back re-opens the first request too.
            await pilot.press("k")
            await pilot.pause()
            assert tree.currently_open is tree.cursor_node
            app.clear_notifications()

    asyncio.run(check())
