import asyncio

import pytest
from test_snapshots import SAMPLE_COLLECTIONS, use_config

from posting.__main__ import make_posting
from posting.app import MainScreen
from posting.posting3_notice import Posting3Notice


def screens_after_startup() -> list[type]:
    async def run() -> list[type]:
        app = make_posting(collection=SAMPLE_COLLECTIONS)
        async with app.run_test() as pilot:
            await pilot.pause()
            return [type(screen) for screen in app.screen_stack]

    return asyncio.run(run())


@use_config("general.yaml")
def test_notice_shown_on_first_launch_and_marker_written(posting3_notice_marker):
    posting3_notice_marker.unlink()

    assert screens_after_startup() == [MainScreen, Posting3Notice]
    assert posting3_notice_marker.exists()


@use_config("general.yaml")
def test_notice_not_shown_when_marker_exists():
    assert screens_after_startup() == [MainScreen]


@use_config("general.yaml")
def test_notice_shown_when_marker_cannot_be_written(tmp_path, monkeypatch):
    not_a_directory = tmp_path / "file"
    not_a_directory.touch()
    monkeypatch.setattr(
        "posting.posting3_notice.data_directory", lambda: not_a_directory
    )

    assert screens_after_startup() == [MainScreen, Posting3Notice]


@use_config("general.yaml")
@pytest.mark.parametrize("key", ["escape", "enter"])
def test_notice_dismissed_with_key(key, posting3_notice_marker):
    posting3_notice_marker.unlink()

    async def run() -> None:
        app = make_posting(collection=SAMPLE_COLLECTIONS)
        async with app.run_test() as pilot:
            await pilot.pause()
            assert isinstance(app.screen, Posting3Notice)
            await pilot.press(key)
            assert isinstance(app.screen, MainScreen)

    asyncio.run(run())


@use_config("general.yaml")
def test_notice_snapshot(posting3_notice_marker, snap_compare):
    posting3_notice_marker.unlink()

    assert snap_compare(
        make_posting(collection=SAMPLE_COLLECTIONS), terminal_size=(100, 34)
    )
