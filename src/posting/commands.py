from functools import partial
from typing import TYPE_CHECKING, cast
from textual.command import DiscoveryHit, Hit, Hits, Provider
from textual.types import IgnoreReturnCallbackType

if TYPE_CHECKING:
    from posting.app import Posting


CommandType = tuple[str, IgnoreReturnCallbackType, str, bool]


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

            if screen.url_bar.url_input.value.strip() != "":
                commands_to_show.append(
                    (
                        "export: copy as curl",
                        app.command_export_to_curl,
                        "Copy the request as a curl command",
                        True,
                    ),
                )

                commands_to_show.append(
                    (
                        "export: copy as curl (no setup scripts)",
                        partial(app.command_export_to_curl, run_setup_scripts=False),
                        "Copy the request as a curl command without setup scripts",
                        True,
                    ),
                )
            # Change the available commands depending on what is currently
            # maximized on the main screen.
            expand_section_callback: IgnoreReturnCallbackType = partial[None](
                screen.expand_section, None
            )
            reset_command: CommandType = (
                "view: Reset",
                expand_section_callback,
                "Reset the size of the request & response sections",
                True,
            )
            expand_request_callback: IgnoreReturnCallbackType = partial[None](
                screen.expand_section, "request"
            )
            expand_request_command: CommandType = (
                "view: Expand request section",
                expand_request_callback,
                "Expand the request section and hide the response section",
                True,
            )
            expand_response_callback: IgnoreReturnCallbackType = partial[None](
                screen.expand_section, "response"
            )
            expand_response_command: CommandType = (
                "view: Expand response section",
                expand_response_callback,
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

            toggle_collection_browser_callback: IgnoreReturnCallbackType = partial[
                None
            ](screen.action_toggle_collection_browser)
            toggle_collection_browser_command: CommandType = (
                "view: Toggle collection browser",
                toggle_collection_browser_callback,
                "Toggle the collection browser sidebar",
                True,
            )
            commands_to_show.append(toggle_collection_browser_command)

        # Global commands, not specific to the MainScreen.
        if not app.ansi_color:
            commands_to_show.append(
                (
                    "theme: Preview theme",
                    app.action_change_theme,
                    "Preview a theme for the current session",
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
