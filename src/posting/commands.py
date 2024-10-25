from functools import partial
from typing import TYPE_CHECKING, cast
from textual.command import DiscoveryHit, Hit, Hits, Provider
from textual.types import IgnoreReturnCallbackType

if TYPE_CHECKING:
    from posting.app import Posting


class PostingProvider(Provider):
    @property
    def commands(
        self,
    ) -> tuple[tuple[str, IgnoreReturnCallbackType, str, bool], ...]:
        app = self.posting
        screen = self.screen

        commands_to_show: list[tuple[str, IgnoreReturnCallbackType, str, bool]] = [
            *self.get_theme_commands(),
            ("app: quit", app.action_quit, "Quit Posting", True),
        ]

        from posting.app import MainScreen

        if isinstance(screen, MainScreen):
            # Only show the option to change to the layout which isn't the current one.
            if screen.current_layout == "horizontal":
                commands_to_show.append(
                    (
                        "layout: vertical",
                        partial(app.command_layout, "vertical"),
                        "Change layout to vertical",
                        True,
                    ),
                )
            elif screen.current_layout == "vertical":
                commands_to_show.append(
                    (
                        "layout: horizontal",
                        partial(app.command_layout, "horizontal"),
                        "Change layout to horizontal",
                        True,
                    ),
                )

            # Change the available commands depending on what is currently
            # maximized on the main screen.
            reset_command = (
                "view: reset",
                partial(screen.expand_section, None),
                "Reset section sizes to default",
                True,
            )
            expand_request_command = (
                "view: expand request",
                partial(screen.expand_section, "request"),
                "Expand the request section",
                True,
            )
            expand_response_command = (
                "view: expand response",
                partial(screen.expand_section, "response"),
                "Expand the response section",
                True,
            )
            expanded_section = screen.expanded_section
            if expanded_section == "request":
                commands_to_show.extend([reset_command, expand_response_command])
            elif expanded_section == "response":
                commands_to_show.extend([reset_command, expand_request_command])
            else:
                commands_to_show.extend(
                    [expand_request_command, expand_response_command]
                )

            commands_to_show.append(
                (
                    "view: toggle collection browser",
                    screen.action_toggle_collection_browser,
                    "Toggle the collection browser",
                    True,
                ),
            )

        return tuple(commands_to_show)

    async def discover(self) -> Hits:
        """Handle a request for the discovery commands for this provider.

        Yields:
            Commands that can be discovered.
        """
        for name, runnable, help_text, show_discovery in self.commands:
            if show_discovery:
                yield DiscoveryHit(
                    name,
                    runnable,
                    help=help_text,
                )

    async def search(self, query: str) -> Hits:
        """Handle a request to search for commands that match the query.

        Args:
            query: The user input to be matched.

        Yields:
            Command hits for use in the command palette.
        """
        matcher = self.matcher(query)
        for name, runnable, help_text, _ in self.commands:
            if (match := matcher.match(name)) > 0:
                yield Hit(
                    match,
                    matcher.highlight(name),
                    runnable,
                    help=help_text,
                )

    def get_theme_commands(
        self,
    ) -> tuple[tuple[str, IgnoreReturnCallbackType, str, bool], ...]:
        posting = self.posting
        return tuple(self.get_theme_command(theme) for theme in posting.themes)

    def get_theme_command(
        self, theme_name: str
    ) -> tuple[str, IgnoreReturnCallbackType, str, bool]:
        return (
            f"theme: {theme_name}",
            partial(self.posting.command_theme, theme_name),
            f"Set the theme to {theme_name}",
            False,
        )

    @property
    def posting(self) -> "Posting":
        return cast("Posting", self.screen.app)
