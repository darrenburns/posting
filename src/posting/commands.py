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

        commands_to_show: list[tuple[str, IgnoreReturnCallbackType, str, bool]] = []

        from posting.app import MainScreen

        if isinstance(screen, MainScreen):
            # Only show the option to change to the layout which isn't the current one.
            if screen.current_layout == "horizontal":
                commands_to_show.append(
                    (
                        "layout: Vertical",
                        partial(app.command_layout, "vertical"),
                        "Change layout to vertical",
                        True,
                    ),
                )
            elif screen.current_layout == "vertical":
                commands_to_show.append(
                    (
                        "layout: Horizontal",
                        partial(app.command_layout, "horizontal"),
                        "Change layout to horizontal",
                        True,
                    ),
                )

            # Change the available commands depending on what is currently
            # maximized on the main screen.
            reset_command = (
                "view: Reset",
                partial(screen.expand_section, None),
                "Reset the size of the request & response sections",
                True,
            )
            expand_request_command = (
                "view: Expand request section",
                partial(screen.expand_section, "request"),
                "Expand the request section and hide the response section",
                True,
            )
            expand_response_command = (
                "view: Expand response section",
                partial(screen.expand_section, "response"),
                "Expand the response section and hide the request section",
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
                    "view: Toggle collection browser",
                    screen.action_toggle_collection_browser,
                    "Toggle the collection browser sidebar",
                    True,
                ),
            )

            if not app.ansi_color:
                commands_to_show.append(
                    (
                        "theme: Change theme",
                        app.action_change_theme,
                        "Change the current theme",
                        True,
                    ),
                )

            if screen.query("HelpPanel"):
                commands_to_show.append(
                    (
                        "help: Hide keybindings sidebar",
                        app.action_hide_help_panel,
                        "Hide the keybindings sidebar",
                        True,
                    ),
                )
            else:
                commands_to_show.append(
                    (
                        "help: Show keybindings sidebar",
                        app.action_show_help_panel,
                        "Display keybindings for the focused widget in a sidebar",
                        True,
                    ),
                )

            commands_to_show.append(
                (
                    "app: Quit Posting",
                    app.action_quit,
                    "Quit Posting and return to the command line",
                    True,
                ),
            )
            
            commands_to_show.append(
                (
                "environment: Show environment variables",
                app.show_environment,
                "Show the environment variables screen",
                True,
                )
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

    @property
    def posting(self) -> "Posting":
        return cast("Posting", self.screen.app)
