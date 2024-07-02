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
    ) -> tuple[tuple[str, IgnoreReturnCallbackType, str], ...]:
        app = self.posting
        screen = self.screen

        commands_to_show: list[tuple[str, IgnoreReturnCallbackType, str]] = [
            *self.get_theme_commands(),
        ]

        from posting.app import MainScreen

        if isinstance(screen, MainScreen):
            # Only show the option to change to the layout which isn't the current one.
            if screen.layout == "horizontal":
                commands_to_show.append(
                    (
                        "layout: vertical",
                        partial(app.command_layout, "vertical"),
                        "Change layout to vertical",
                    ),
                )
            elif screen.layout == "vertical":
                commands_to_show.append(
                    (
                        "layout: horizontal",
                        partial(app.command_layout, "horizontal"),
                        "Change layout to horizontal",
                    ),
                )

            # Change the available commands depending on what is currently
            # maximized on the main screen.
            maximized = screen.maximized
            reset_command = (
                "view: reset",
                partial(screen.maximize_section, None),
                "Reset section sizes to default",
            )
            expand_request_command = (
                "view: expand request",
                partial(screen.maximize_section, "request"),
                "Expand the request section",
            )
            expand_response_command = (
                "view: expand response",
                partial(screen.maximize_section, "response"),
                "Expand the response section",
            )
            if maximized == "request":
                commands_to_show.extend([reset_command, expand_response_command])
            elif maximized == "response":
                commands_to_show.extend([reset_command, expand_request_command])
            else:
                commands_to_show.extend(
                    [expand_request_command, expand_response_command]
                )

        return tuple(commands_to_show)

    async def discover(self) -> Hits:
        """Handle a request for the discovery commands for this provider.

        Yields:
            Commands that can be discovered.
        """
        for name, runnable, help_text in self.commands:
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
        for name, runnable, help_text in self.commands:
            if (match := matcher.match(name)) > 0:
                yield Hit(
                    match,
                    matcher.highlight(name),
                    runnable,
                    help=help_text,
                )

    def get_theme_commands(
        self,
    ) -> tuple[tuple[str, IgnoreReturnCallbackType, str], ...]:
        posting = self.posting
        return tuple(self.get_theme_command(theme) for theme in posting.themes)

    def get_theme_command(
        self, theme_name: str
    ) -> tuple[str, IgnoreReturnCallbackType, str]:
        return (
            f"theme: {theme_name}",
            partial(self.posting.command_theme, theme_name),
            f"Set the theme to {theme_name}",
        )

    @property
    def posting(self) -> "Posting":
        return cast("Posting", self.screen.app)
