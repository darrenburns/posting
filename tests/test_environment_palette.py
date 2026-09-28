from textual.command import CommandPalette
from posting.__main__ import make_posting
from posting.environment_commands import EnvironmentProvider
from posting.environments import switch_environment
from posting.variables import get_variables
from test_snapshots import use_config, disable_blink_for_active_cursors


@use_config("general.yaml")
def test_environment_submenu(tmp_path, monkeypatch, snap_compare):
    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "config"))
    (tmp_path / "development.env").write_text("MODE=development\nDEV_ONLY=yes\n")
    (tmp_path / "production.env").write_text("MODE=production\n")
    app = make_posting(collection=tmp_path, env=(str(tmp_path / "development.env"),))

    async def run_before(pilot):
        await pilot.pause()
        await pilot.press("ctrl+p")
        await pilot.press(*"switch environment")
        await pilot.pause()
        await pilot.press("down", "enter")
        await pilot.pause()
        assert isinstance(app.screen, CommandPalette)
        assert any(
            isinstance(provider, EnvironmentProvider)
            for provider in app.screen._providers
        )
        await disable_blink_for_active_cursors(pilot)
        # Keep the screenshot independent of pytest's temporary directory.
        app.screen.query_one("CommandInput").value = "production"
        await pilot.pause()
        await pilot.press("down", "enter")
        await pilot.pause()
        assert get_variables() == {"MODE": "production"}
        assert app.environment_files == ((tmp_path / "production.env").resolve(),)
        app.session_env["SESSION"] = "kept"
        assert switch_environment(app, ())
        assert get_variables() == {"SESSION": "kept"}
        assert switch_environment(app, (tmp_path / "development.env",))
        assert get_variables()["MODE"] == "development"
        assert not switch_environment(app, (tmp_path / "missing.env",))
        assert get_variables()["MODE"] == "development"
        await pilot.pause()
        app.clear_notifications()
        app.screen.collection_browser.border_subtitle = "collection"
        await pilot.press("ctrl+p")
        await pilot.press(*"switch environment")
        await pilot.press("down", "enter")
        await pilot.pause()
        await disable_blink_for_active_cursors(pilot)
        # Hide absolute help paths in this snapshot only.
        app.screen.query_one("CommandInput").value = "No environment"
        await pilot.pause()
        app.clear_notifications()

    assert snap_compare(app, run_before=run_before, terminal_size=(120, 40))


@use_config("general.yaml")
def test_switch_environment_restarts_watcher(tmp_path):
    import asyncio

    first = tmp_path / "first.env"
    second = tmp_path / "second.env"
    first.write_text("MODE=first\n")
    second.write_text("MODE=second\n")
    app = make_posting(collection=tmp_path, env=(str(first),))
    app.settings.watch_env_files = True

    async def verify():
        async with app.run_test() as pilot:
            await pilot.pause()
            assert switch_environment(app, (second,))
            await pilot.pause(0.3)
            second.write_text("MODE=updated\n")
            for _ in range(20):
                await pilot.pause(0.1)
                if get_variables().get("MODE") == "updated":
                    break
            assert get_variables()["MODE"] == "updated"
            assert switch_environment(app, ())
            await pilot.pause(0.2)
            assert not [
                w
                for w in app.workers
                if w.group == "environment-watcher" and w.is_running
            ]

    asyncio.run(verify())
