import asyncio
import shutil
from pathlib import Path

from posting.__main__ import make_posting
from posting.collection import RequestModel
from posting.widgets.collection.browser import CollectionTree


def test_saving_deleted_open_request_restores_tree_node(
    tmp_path: Path,
    monkeypatch,
) -> None:
    collection_path = tmp_path / "collection"
    shutil.copytree(
        Path(__file__).parent / "sample-collections",
        collection_path,
    )

    monkeypatch.setenv(
        "POSTING_CONFIG_FILE",
        str(Path(__file__).parent / "sample-configs" / "general.yaml"),
    )
    monkeypatch.delenv("NO_COLOR", raising=False)

    async def run_test() -> None:
        app = make_posting(collection=collection_path, env=())

        async with app.run_test() as pilot:
            await pilot.pause()

            tree = app.screen.query_one(CollectionTree)
            request_node = next(
                child
                for child in tree.root.children
                if isinstance(child.data, RequestModel)
            )
            request_path = request_node.data.path
            assert request_path is not None

            tree.select_node(request_node)
            await pilot.pause()

            tree.cursor_line = request_node.line
            tree.action_delete_request()
            await pilot.pause()

            assert not request_path.exists()
            assert request_node not in tree.root.children

            await app.screen.action_save_request()
            await pilot.pause()

            assert request_path.exists()
            assert any(
                isinstance(child.data, RequestModel)
                and child.data.path == request_path
                for child in tree.root.children
            )
            assert tree.currently_open in tree.root.children

    asyncio.run(run_test())
