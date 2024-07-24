import os
from pathlib import Path
from unittest import mock
import pytest

from textual.pilot import Pilot
from textual.widgets import Input
from posting.__main__ import make_posting

TEST_DIR = Path(__file__).parent
CONFIG_DIR = TEST_DIR / "sample-configs"
ENV_DIR = TEST_DIR / "sample-envs"
THEME_DIR = TEST_DIR / "sample-themes"
SAMPLE_COLLECTIONS = TEST_DIR / "sample-collections"
POSTING_MAIN = TEST_DIR / "posting_snapshot_app.py"


def use_config(file_name: str):
    """Specify which config file to use from the `sample-configs` directory."""
    return mock.patch.dict(
        os.environ, {"POSTING_CONFIG_FILE": str(CONFIG_DIR / file_name)}
    )


def patch_env(key: str, value: str):
    """Decorator to patch and environment variable."""
    return mock.patch.dict(os.environ, {key: value})


async def disable_blink_for_active_cursors(pilot: Pilot) -> None:
    """Allows us to disable cursors which could not be disabled via config.
    You'll probably want to use this if you open the command palette using a
    test, as the config does not target that Input's cursor.
    """
    await pilot.pause()
    pilot.app.screen.query_one(Input).cursor_blink = False


@use_config("general.yaml")
class TestJumpMode:
    def test_loads(self, snap_compare):
        """Simple check that ctrl+o enters jump mode."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+o")

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_focus_switch(self, snap_compare):
        """Jump mode can target focusable widgets such as the collection tree."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+o")  # enter jump mode
            await pilot.press("tab")  # target collection tree

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_click_switch(self, snap_compare):
        """Jump mode can target widgets that are not focusable, such as tabs."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+o")  # enter jump mode
            await pilot.press("y")  # target "Options" tab

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
class TestMethodSelection:
    def test_select_post_method(self, snap_compare):
        """Simple check that we can change the method."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+t")
            await pilot.press("p")
            await pilot.press("enter")

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
class TestUrlBar:
    def test_dropdown_appears_on_typing(self, snap_compare):
        """Check that the dropdown is filled with URLs."""

        async def run_before(pilot: Pilot):
            await pilot.pause()
            await pilot.press(*"http")  # Move to the dropdown

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_dropdown_filters_on_typing(self, snap_compare):
        """Check that the dropdown filters on typing."""

        async def run_before(pilot: Pilot):
            await pilot.pause()
            await pilot.press(*"json")  # Move to the dropdown

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_dropdown_completion_selected_via_enter_key(self, snap_compare):
        """Check that the dropdown completion is selected."""

        async def run_before(pilot: Pilot):
            await pilot.pause()
            await pilot.press(*"json")  # Move to the dropdown
            await pilot.press("enter")  # Select the completion

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_dropdown_completion_selected_via_tab_key(self, snap_compare):
        """Check that the dropdown completion is selected."""

        async def run_before(pilot: Pilot):
            await pilot.pause()
            await pilot.press(*"json")  # Move to the dropdown
            await pilot.press("tab")  # Select the completion

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
class TestCommandPalette:
    def test_loads_and_shows_discovery_options(self, snap_compare):
        """Check that the command palette loads."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+p")
            await disable_blink_for_active_cursors(pilot)

        assert snap_compare(
            POSTING_MAIN, run_before=run_before, terminal_size=(120, 34)
        )

    def test_can_type_to_filter_options(self, snap_compare):
        """Check that we can run a command from the command palette."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+p")
            await disable_blink_for_active_cursors(pilot)
            await pilot.press(*"view")

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_can_run_command__hide_collection_browser(self, snap_compare):
        """Check that we can run a command from the command palette."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+p")
            await disable_blink_for_active_cursors(pilot)
            await pilot.press(*"toggle collection")
            await pilot.press("enter", "enter")

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
@patch_env("POSTING_FOCUS__ON_STARTUP", "collection")
class TestNewRequest:
    def test_dialog_loads_and_can_be_used(self, snap_compare):
        """Check that the new request dialog loads and is prefilled
        with the data based on where the cursor is."""

        async def run_before(pilot: Pilot):
            await pilot.press("J", "J", "ctrl+n")
            await pilot.press(*"foo")
            await pilot.press("tab", "tab")
            await pilot.press(*"bar")

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    @pytest.mark.serial
    def test_new_request_added_to_tree_correctly_and_notification_shown(
        self, snap_compare
    ):
        """Check that the new request is added to the tree correctly
        and that a notification is shown."""

        async def run_before(pilot: Pilot):
            await pilot.press("J", "J", "ctrl+n")
            await pilot.press(*"foo")
            await pilot.press("tab", "tab")
            await pilot.press(*"bar")
            await pilot.press("ctrl+n")

            await pilot.pause()
            # Check the file exists
            new_request_file = (
                SAMPLE_COLLECTIONS / "jsonplaceholder" / "posts" / "foo.posting.yaml"
            )
            assert new_request_file.exists()

            # Check the file content
            assert (
                new_request_file.read_text("utf-8")
                == """\
name: foo
description: bar
"""
            )

            new_request_file.unlink()

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
class TestUserInterfaceShortcuts:
    def test_hide_collection_browser(self, snap_compare):
        """Check that we can hide the collection browser."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+h")

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_expand_request_section(self, snap_compare):
        """Check that we can expand the request section."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+o")  # jump mode
            await pilot.press("q")  # move focus to inside request section
            await pilot.press("ctrl+m")  # expand request section

        assert snap_compare(POSTING_MAIN, run_before=run_before)

    def test_expand_then_reset(self, snap_compare):
        """Check that we can expand the request section and then reset it."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+o")  # jump mode
            await pilot.press("q")  # move focus to inside request section
            await pilot.press("ctrl+m")  # expand request section
            await pilot.press("ctrl+m")  # reset sections

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
@patch_env("POSTING_FOCUS__ON_STARTUP", "collection")
class TestLoadingRequest:
    def test_request_loaded_into_view__headers(self, snap_compare):
        """Check that the request headers are loaded into the view."""

        async def run_before(pilot: Pilot):
            # Navigate to 'get one post' and select it.
            await pilot.press(*"Jj")
            await pilot.press("enter")

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 34))

    def test_request_loaded_into_view__body(self, snap_compare):
        """Check that the request body is loaded into the view."""

        async def run_before(pilot: Pilot):
            # Navigate to 'POST one post' and select it.
            await pilot.press(*"JJjjj")
            await pilot.press("enter")
            await pilot.press("ctrl+o", "w")  # jump to 'Body' tab

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 34))

    def test_request_loaded_into_view__query_params(self, snap_compare):
        """Check that the request query params are loaded into the view."""

        async def run_before(pilot: Pilot):
            # Navigate to 'GET comments via query' and select it.
            await pilot.press(*"JJJjj")
            await pilot.press("enter")
            await pilot.press("ctrl+o", "e")  # jump to 'Query Params' tab

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 34))

    def test_request_loaded_into_view__auth(self, snap_compare):
        """Check that the request auth is loaded into the view."""

        async def run_before(pilot: Pilot):
            # Navigate to 'GET comments via query' and select it.
            await pilot.press(*"jj")
            await pilot.press("enter")
            await pilot.press("ctrl+o", "r")  # jump to 'Auth' tab

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 44))

    @pytest.mark.skip(
        reason="info tab contains a path, specific to the host the test runs on"
    )
    def test_request_loaded_into_view__info(self, snap_compare):
        """Check that the request info is loaded into the view."""

        async def run_before(pilot: Pilot):
            await pilot.press("j")
            await pilot.press("enter")
            await pilot.press("ctrl+o", "t")  # jump to 'Info' tab

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 44))

    def test_request_loaded_into_view__options(self, snap_compare):
        """Check that the request options are loaded into the view."""

        async def run_before(pilot: Pilot):
            await pilot.press(*"jj")
            await pilot.press("enter")
            await pilot.press("ctrl+o", "y")  # jump to 'Options' tab

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 44))


@use_config("general.yaml")
class TestHelpScreen:
    def test_help_screen_appears(self, snap_compare):
        """Check that the help screen appears."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+question_mark")

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(80, 42))


@use_config("general.yaml")
@patch_env("POSTING_FOCUS__ON_STARTUP", "collection")
class TestSave:
    def test_no_request_selected__dialog_is_prefilled_correctly(self, snap_compare):
        """Check that the save dialog appears when no request is selected.

        We should confirm that the New Request dialogue is open, and that the
        information is pre-filled. Ensure that the filename is also computed
        correctly for the `*.posting.yaml` file.
        """

        async def run_before(pilot: Pilot):
            await pilot.press(*"JJj")
            await pilot.press("ctrl+o", "t")  # select 'Info' tab
            await pilot.press("j")  # move down into 'Info' tab
            await pilot.press(*"Foo: Bar")
            await pilot.press("tab")
            await pilot.press(*"baz")
            await pilot.press("ctrl+s")

        assert snap_compare(POSTING_MAIN, run_before=run_before)


@use_config("general.yaml")
@patch_env("POSTING_FOCUS__ON_STARTUP", "collection")
class TestSendRequest:
    def test_send_request(self, snap_compare):
        """Check that we can send a request."""

        async def run_before(pilot: Pilot):
            await pilot.press(*"JJj")  # select 'get one post'
            await pilot.press("l")  # testing 'l' to select
            await pilot.press("ctrl+j")  # send request
            await pilot.app.workers.wait_for_complete()

        assert snap_compare(POSTING_MAIN, run_before=run_before, terminal_size=(88, 34))


@use_config("modified_config.yaml")
class TestConfig:
    def test_config(self, snap_compare):
        """Check that the config is loaded correctly.
        The config loaded in this test class modifies the theme,
        layout, focus on startup and response, and hides the header.

        We should pay special attention to ensure the response body
        is focused, as it should happen automatically when the response
        is received based on this config.
        """

        async def run_before(pilot: Pilot):
            await pilot.press(*"JJj")
            await pilot.press("l")
            await pilot.press("ctrl+j")
            await pilot.app.workers.wait_for_complete()

        assert snap_compare(
            POSTING_MAIN, run_before=run_before, terminal_size=(120, 34)
        )


@use_config("general.yaml")
@patch_env("POSTING_FOCUS__ON_STARTUP", "collection")
class TestVariables:
    def test_unresolved_variables_highlighted(self, snap_compare):
        """Check that the unresolved variables are highlighted in the URL
        and as a query parameter value."""

        async def run_before(pilot: Pilot):
            await pilot.press(*"JJJJj")  # go to 'get one user'
            await pilot.press("enter")  # press 'enter' to select
            await pilot.press("ctrl+o", "e")  # go to 'Query' tab
            await pilot.press("down")  # move down into 'Query' tab
            await pilot.press(*"foo", "enter")  # pressing enter should shift to value
            # The params typed below should be dimmed since they dont resolve
            await pilot.press(*"$nope/${nope}")

        assert snap_compare(
            POSTING_MAIN, run_before=run_before, terminal_size=(118, 34)
        )

    def test_resolved_variables_highlight_and_preview(self, snap_compare):
        """Check that the resolved variables are highlighted in the URL
        and the value is shown below."""

        env_path = str((ENV_DIR / "sample_base.env").resolve())
        app = make_posting(
            collection=SAMPLE_COLLECTIONS / "jsonplaceholder" / "todos",
            env=(env_path,),
        )

        async def run_before(pilot: Pilot):
            await pilot.press("j", "j", "enter")
            await pilot.press("ctrl+l")

        assert snap_compare(app, run_before=run_before)


@use_config("custom_theme.yaml")
@patch_env("POSTING_THEME_DIRECTORY", str(THEME_DIR.resolve()))
class TestCustomTheme:
    def test_theme_set_on_startup_and_in_command_palette(self, snap_compare):
        """Check that the theme is set on startup and available in the command palette."""

        async def run_before(pilot: Pilot):
            await pilot.press("ctrl+p")
            await disable_blink_for_active_cursors(pilot)
            await pilot.press(*"anothertest")

        assert snap_compare(POSTING_MAIN, run_before=run_before)
