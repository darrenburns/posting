"""Tests for the MethodSelector widget keyboard bindings.

These tests verify that single-letter keyboard shortcuts work correctly
when the dropdown is both closed and expanded.
"""

import pytest
from textual.app import App, ComposeResult
from textual.widgets import Select

from posting.widgets.request.method_selection import MethodSelector


@pytest.fixture
def anyio_backend():
    return "asyncio"


pytestmark = pytest.mark.anyio


class MethodSelectorApp(App):
    """A minimal app for testing the MethodSelector widget."""

    def compose(self) -> ComposeResult:
        yield MethodSelector(id="method-selector", value="GET")


class TestMethodSelectorKeyBindings:
    """Test that keyboard bindings work for all HTTP methods."""

    @pytest.fixture
    def app(self):
        return MethodSelectorApp()

    @pytest.mark.parametrize(
        "key,expected_method",
        [
            ("g", "GET"),
            ("p", "POST"),
            ("u", "PUT"),
            ("d", "DELETE"),
            ("a", "PATCH"),
            ("h", "HEAD"),
            ("o", "OPTIONS"),
        ],
    )
    async def test_keybinding_works_when_dropdown_closed(
        self, app, key, expected_method
    ):
        """Test that single-letter shortcuts work when the dropdown is closed."""
        async with app.run_test() as pilot:
            selector = app.query_one(MethodSelector)
            selector.focus()

            # Ensure dropdown is closed
            assert not selector.expanded

            # Press the shortcut key
            await pilot.press(key)

            # Verify the method was selected
            assert selector.value == expected_method

    @pytest.mark.parametrize(
        "key,expected_method",
        [
            ("g", "GET"),
            ("p", "POST"),
            ("u", "PUT"),
            ("d", "DELETE"),
            ("a", "PATCH"),
            ("h", "HEAD"),
            ("o", "OPTIONS"),
        ],
    )
    async def test_keybinding_works_when_dropdown_expanded(
        self, app, key, expected_method
    ):
        """Test that single-letter shortcuts work when the dropdown is expanded.

        This is the critical test case - previously, when the dropdown was expanded,
        the SelectOverlay's type-to-search feature would intercept the key presses
        and prevent the shortcuts from working.
        """
        async with app.run_test() as pilot:
            selector = app.query_one(MethodSelector)
            selector.focus()

            # Open the dropdown
            await pilot.press("enter")
            await pilot.pause()

            # Verify dropdown is expanded
            assert selector.expanded

            # Press the shortcut key while dropdown is open
            await pilot.press(key)

            # Verify the method was selected
            assert selector.value == expected_method

    async def test_all_methods_selectable_in_sequence_while_expanded(self, app):
        """Test rapidly switching between all methods while dropdown stays expanded."""
        async with app.run_test() as pilot:
            selector = app.query_one(MethodSelector)
            selector.focus()

            # Open the dropdown
            await pilot.press("enter")
            await pilot.pause()
            assert selector.expanded

            # Cycle through all methods
            methods = [
                ("g", "GET"),
                ("p", "POST"),
                ("u", "PUT"),
                ("d", "DELETE"),
                ("a", "PATCH"),
                ("h", "HEAD"),
                ("o", "OPTIONS"),
            ]

            for key, expected in methods:
                await pilot.press(key)
                assert selector.value == expected, f"Expected {expected} after pressing '{key}'"


class TestMethodSelectorTypeToSearchDisabled:
    """Test that type-to-search is disabled for MethodSelector."""

    async def test_type_to_search_is_disabled(self):
        """Verify that type_to_search is False on the MethodSelector.

        This is the root cause fix - when type_to_search is True (default),
        the SelectOverlay intercepts printable characters for search,
        preventing the single-letter shortcuts from working.
        """
        app = MethodSelectorApp()
        async with app.run_test():
            selector = app.query_one(MethodSelector)
            assert selector._type_to_search is False

    async def test_typing_does_not_trigger_search_when_expanded(self):
        """Test that typing characters doesn't trigger search behavior.

        When type_to_search is disabled, typing should trigger the keybindings
        instead of searching through options.
        """
        app = MethodSelectorApp()
        async with app.run_test() as pilot:
            selector = app.query_one(MethodSelector)
            selector.focus()

            # Start with POST
            await pilot.press("p")
            assert selector.value == "POST"

            # Open dropdown
            await pilot.press("enter")
            await pilot.pause()

            # Type 'g' - should select GET, not search for options starting with 'g'
            await pilot.press("g")
            assert selector.value == "GET"

            # Type 'o' - should select OPTIONS, not search for options starting with 'go'
            await pilot.press("o")
            assert selector.value == "OPTIONS"


class TestMethodSelectorInitialValue:
    """Test that initial values are respected."""

    @pytest.mark.parametrize("initial_value", ["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"])
    async def test_initial_value_is_set(self, initial_value):
        """Test that the selector respects the initial value parameter."""

        class TestApp(App):
            def compose(self) -> ComposeResult:
                yield MethodSelector(id="method-selector", value=initial_value)

        app = TestApp()
        async with app.run_test():
            selector = app.query_one(MethodSelector)
            assert selector.value == initial_value
