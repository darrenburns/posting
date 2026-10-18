"""Collection-supplied scripts must not run without explicit user consent.

Regression tests for the untrusted-collection RCE where a `.posting.yaml` in a
collection could name a Python file that Posting imported and executed with the
user's privileges before any confirmation prompt was shown.
"""

import asyncio
from pathlib import Path

from posting.__main__ import make_posting
from posting.scripts import clear_module_cache
from posting.widgets.confirmation import ConfirmationModal


def build_collection(root: Path, canary: Path, *, in_setup: bool = False) -> None:
    (root / "scripts").mkdir(parents=True, exist_ok=True)
    (root / "probe.posting.yaml").write_text(
        "name: probe\n"
        "method: GET\n"
        "url: http://127.0.0.1:9/probe\n"
        "scripts:\n"
        "  setup: scripts/probe.py\n"
    )
    if in_setup:
        body = (
            "from pathlib import Path\n"
            "\n"
            "def setup(posting):\n"
            f"    Path(r'{canary}').write_text('script executed')\n"
        )
    else:
        body = (
            "from pathlib import Path\n"
            "\n"
            "# Executes at import time, before any named function is called.\n"
            f"Path(r'{canary}').write_text('script executed')\n"
            "\n"
            "def setup(posting):\n"
            "    pass\n"
        )
    (root / "scripts" / "probe.py").write_text(body)


async def open_and_send(app, pilot):
    await pilot.pause()
    main_screen = app.main_screen
    request = app.collection.requests[0]
    main_screen.load_request_model(request)
    await pilot.pause()
    main_screen.send_via_worker()
    await pilot.pause()
    await pilot.pause()


def test_script_blocked_without_consent(tmp_path, monkeypatch) -> None:
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "config"))
    monkeypatch.setenv("XDG_DATA_HOME", str(tmp_path / "data"))
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path / "state"))
    clear_module_cache()

    collection = tmp_path / "coll"
    canary = tmp_path / "canary_blocked"
    build_collection(collection, canary)

    async def check():
        app = make_posting(collection=collection)
        async with app.run_test() as pilot:
            await open_and_send(app, pilot)
            # A confirmation prompt must be shown before any code runs.
            assert isinstance(app.screen, ConfirmationModal)
            assert not canary.exists()
            # Decline the prompt.
            await pilot.press("n")
            await pilot.pause()

    asyncio.run(check())
    assert not canary.exists(), "script ran even though the user declined"


def test_script_runs_after_consent(tmp_path, monkeypatch) -> None:
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "config"))
    monkeypatch.setenv("XDG_DATA_HOME", str(tmp_path / "data"))
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path / "state"))
    clear_module_cache()

    collection = tmp_path / "coll"
    canary = tmp_path / "canary_allowed"
    build_collection(collection, canary)

    async def check():
        app = make_posting(collection=collection)
        async with app.run_test() as pilot:
            await open_and_send(app, pilot)
            assert isinstance(app.screen, ConfirmationModal)
            assert not canary.exists()
            # Approve the prompt.
            await pilot.press("y")
            await pilot.pause()
            await pilot.pause()

    asyncio.run(check())
    assert canary.exists(), "script did not run after the user approved it"


def test_approved_script_not_reprompted(tmp_path, monkeypatch) -> None:
    """The same unmodified script is not re-prompted after approval."""
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "config"))
    monkeypatch.setenv("XDG_DATA_HOME", str(tmp_path / "data"))
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path / "state"))
    clear_module_cache()

    collection = tmp_path / "coll"
    canary = tmp_path / "canary_twice"
    build_collection(collection, canary, in_setup=True)

    async def check():
        app = make_posting(collection=collection)
        async with app.run_test() as pilot:
            await open_and_send(app, pilot)
            assert isinstance(app.screen, ConfirmationModal)
            await pilot.press("y")
            await pilot.pause()
            await pilot.pause()
            assert canary.exists()

            # Second run of the same, unmodified script must not prompt again,
            # but the approved setup function still runs.
            canary.unlink()
            app.main_screen.send_via_worker()
            await pilot.pause()
            await pilot.pause()
            assert not isinstance(app.screen, ConfirmationModal)
            assert canary.exists()

    asyncio.run(check())
