from pathlib import Path
from typing import Literal

from rich.console import RenderableType
from textual.content import Content

from textual import messages, on, work
from textual.command import CommandPalette
from textual.app import App, InvalidThemeError, ReturnType
from textual.binding import Binding
from textual.signal import Signal
from textual.theme import Theme, BUILTIN_THEMES as TEXTUAL_THEMES
from watchfiles import Change, awatch
from posting.collection import (
    Collection,
)

from posting.commands import PostingProvider
from posting.config import SETTINGS, Settings
from posting.help_screen import HelpScreen
from posting.http_screen import HttpScreen
from posting.scripts import uncache_module
from posting.themes import (
    BUILTIN_THEMES,
    load_user_theme,
    load_user_themes,
)
from posting.variables import update_variables
from posting.variables import load_variables


from posting.xresources import load_xresources_themes


class Posting(App[None], inherit_bindings=False):
    AUTO_FOCUS = None
    COMMANDS = {PostingProvider}
    CSS_PATH = Path(__file__).parent / "posting.scss"
    BINDING_GROUP_TITLE = "Global Keybinds"
    BINDINGS = [
        Binding(
            "ctrl+p",
            "command_palette",
            description="Commands",
            tooltip="Open the command palette to search and run commands.",
            id="commands",
        ),
        Binding(
            "ctrl+q",
            "app.quit",
            description="Quit",
            tooltip="Quit the application.",
            priority=True,
            id="quit",
        ),
        Binding(
            "f1,ctrl+question_mark",
            "help",
            "Help",
            tooltip="Open the help dialog for the currently focused widget.",
            id="help",
        ),
        Binding("f8", "save_screenshot", "Save screenshot.", show=False),
    ]

    def __init__(
        self,
        settings: Settings,
        environment_files: tuple[Path, ...],
        collection: Collection,
        collection_specified: bool = False,
    ) -> None:
        SETTINGS.set(settings)

        self.settings = settings
        """Settings object which is built via pydantic-settings,
        essentially a direct translation of the config.yaml file."""

        self.environment_files = environment_files
        """A list of paths to dotenv files, in the order they're loaded."""

        self.collection = collection
        """The loaded collection."""

        self.collection_specified = collection_specified
        """Boolean indicating whether the user launched Posting explicitly
        supplying a collection directory, or if they let Posting auto-discover
        it in some way (likely just using the default collection)."""

        self.env_changed_signal = Signal[None](self, "env-changed")
        """Signal that is published when the environment has changed.
        This means one or more of the loaded environment files (in
        `self.environment_files`) have been modified."""

        self.session_env: dict[str, object] = {}
        """Users can set the value of variables for the duration of the
        session (until the app is quit). This can be done via the scripting
        interface: pre-request or post-response scripts."""

        super().__init__()

        # The animation is set AFTER the app is initialized intentionally,
        # as it needs to override the default approach taken by Textual in
        # App.__init__().
        self.animation_level = settings.animation
        """The level of animation to use in the app. This is used by Textual."""

    @work(exclusive=True, group="environment-watcher")
    async def watch_environment_files(self) -> None:
        """Watching files that were passed in as the environment."""
        async for changes in awatch(*self.environment_files):
            # Reload the variables from the environment files.
            load_variables(
                self.environment_files,
                self.settings.use_host_environment,
                avoid_cache=True,
            )
            # Overlay the session variables on top of the environment variables.
            update_variables(self.session_env)

            # Notify the app that the environment has changed,
            # which will trigger a reload of the variables in the relevant widgets.
            # Widgets subscribed to this signal can reload as needed.
            # For example, AutoComplete dropdowns will want to reload their
            # candidate variables when the environment changes.
            self.env_changed_signal.publish(None)
            self.notify(
                title="Environment changed",
                message=f"Reloaded {len(changes)} dotenv files",
                timeout=3,
            )

    @work(exclusive=True, group="collection-watcher")
    async def watch_collection_files(self) -> None:
        """Watching specific files within the collection directory."""
        async for changes in awatch(self.collection.path):
            for change_type, file_path in changes:
                if file_path.endswith(".py"):
                    if change_type in (
                        Change.deleted,
                        Change.modified,
                    ):
                        # If a Python file was updated, then we want to clear
                        # the script module cache for the app so that modules
                        # are reloaded on the next request being sent.
                        # Without this, we'd hit the module cache and simply
                        # re-execute the previously cached module.
                        uncache_module(file_path)
                        file_path_object = Path(file_path)
                        file_name = file_path_object.name
                        self.notify(
                            f"Reloaded {file_name!r}",
                            title="Script reloaded",
                            timeout=2,
                        )
                    if change_type in (Change.added, Change.deleted):
                        # TODO - update the autocompletion
                        # of the available scripts.
                        pass

    @work(exclusive=True, group="theme-watcher")
    async def watch_themes(self) -> None:
        """Watching the theme directory for changes."""
        async for changes in awatch(self.settings.theme_directory):
            for change_type, file_path in changes:
                if file_path.endswith((".yml", ".yaml")):
                    try:
                        theme = load_user_theme(Path(file_path))
                    except Exception as e:
                        print(f"Couldn't load theme from {str(file_path)}: {e}.")
                        continue
                    if theme and theme.name == self.theme:
                        self.register_theme(theme)
                        self.set_reactive(App.theme, theme.name)
                        try:
                            self._watch_theme(theme.name)
                        except Exception as e:
                            # I don't think we want to notify here, as editors often
                            # use heuristics to determine whether to save a file. This could
                            # prove jarring if we pop up a notification without the user
                            # explicitly saving the file in their editor.
                            print(f"Error refreshing CSS: {e}")

    def on_mount(self) -> None:
        settings = SETTINGS.get()
        available_themes: dict[str, Theme] = {"galaxy": BUILTIN_THEMES["galaxy"]}

        if settings.load_builtin_themes:
            available_themes |= BUILTIN_THEMES
        else:
            for theme in TEXTUAL_THEMES.values():
                self.unregister_theme(theme.name)

        if settings.use_xresources:
            available_themes |= load_xresources_themes()

        if settings.load_user_themes:
            loaded_themes, failed_themes = load_user_themes()
            available_themes |= loaded_themes

            # Display a single message for all failed themes.
            if failed_themes:
                self.notify(
                    "\n".join(f"• {path.name}" for path, _ in failed_themes),
                    title=f"Failed to read {len(failed_themes)} theme{'s' if len(failed_themes) > 1 else ''}",
                    severity="error",
                    timeout=8,
                )

        for theme in available_themes.values():
            self.register_theme(theme)

        unwanted_themes = [
            "textual-ansi",
        ]
        for theme_name in unwanted_themes:
            self.unregister_theme(theme_name)

        try:
            self.theme = settings.theme
        except InvalidThemeError:
            # This can happen if the user has a custom theme that is invalid,
            # e.g. a color is invalid or the YAML cannot be parsed.
            self.theme = "galaxy"
            self.notify(
                "Check theme file for syntax errors, invalid colors, etc.\n"
                "Falling back to [b i]galaxy[/] theme.",
                title=f"Couldn't apply theme {settings.theme!r}",
                severity="error",
                timeout=8,
            )

        self.set_keymap(self.settings.keymap)

        if self.settings.watch_env_files:
            self.watch_environment_files()

        if self.settings.watch_collection_files:
            self.watch_collection_files()

        if self.settings.watch_themes:
            self.watch_themes()

    def command_layout(self, layout: Literal["vertical", "horizontal"]) -> None:
        self.http_screen.current_layout = layout

    def action_save_screenshot(
        self,
    ) -> str:
        return self.save_screenshot()

    @on(CommandPalette.Opened)
    def palette_opened(self) -> None:
        # If the theme preview is disabled, don't record the theme being used
        # before the palette is opened.
        if not self.settings.command_palette.theme_preview:
            return

        # Record the theme being used before the palette is opened.
        self._original_theme = self.theme

    @on(CommandPalette.OptionHighlighted)
    def palette_option_highlighted(
        self, event: CommandPalette.OptionHighlighted
    ) -> None:
        # If the theme preview is disabled, don't update the theme when an option
        # is highlighted.
        if not self.settings.command_palette.theme_preview:
            return

        prompt = event.highlighted_event.option.prompt
        themes = self.available_themes.keys()
        if isinstance(prompt, Content):
            candidate = prompt.plain
            if candidate in themes:
                self.theme = candidate
            else:
                self.theme = self._original_theme
            self.call_next(self.screen._update_styles)

    @on(CommandPalette.Closed)
    def palette_closed(self, event: CommandPalette.Closed) -> None:
        # If we closed with a result, that will be handled by the command
        # being triggered. However, if we closed the palette with no result
        # then make sure we revert the theme back.
        if not self.settings.command_palette.theme_preview:
            return
        if not event.option_selected:
            self.theme = self._original_theme

    def get_default_screen(self) -> HttpScreen:
        return HttpScreen(
            collection=self.collection,
            layout=self.settings.layout,
            environment_files=self.environment_files,
        )

    async def action_help(self) -> None:
        focused = self.focused
        if focused is None:
            return

        def reset_focus(_) -> None:
            if focused:
                self.screen.set_focus(focused)

        self.set_focus(None)
        await self.push_screen(HelpScreen(widget=focused), callback=reset_focus)

    def exit(
        self,
        result: ReturnType | None = None,
        return_code: int = 0,
        message: RenderableType | None = None,
    ) -> None:
        """Exit the app, and return the supplied result.

        Args:
            result: Return value.
            return_code: The return code. Use non-zero values for error codes.
            message: Optional message to display on exit.
        """
        self._exit = True
        self._return_value = result
        self._return_code = return_code
        self.post_message(messages.ExitApp())
        if message:
            self._exit_renderables.append(message)
            self._exit_renderables = list(set(self._exit_renderables))
