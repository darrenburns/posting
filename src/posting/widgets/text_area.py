import os
import shlex
import subprocess
import tempfile
from dataclasses import dataclass
from typing import Iterable

from textual import events, on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.message import Message
from textual.reactive import Reactive, reactive
from textual.theme import Theme as TextualTheme
from textual.widgets import Checkbox, Label, Select, TextArea
from textual.widgets.text_area import (
    Location,
    Selection,
    TextAreaTheme,
    ThemeDoesNotExist,
)
from typing_extensions import Literal

from posting.config import SETTINGS
from posting.exit_codes import GENERAL_ERROR
from posting.themes import Theme
from posting.widgets.select import PostingSelect


class TextAreaFooter(Horizontal):
    """The bar that appears above the response body, allowing
    you to customise the syntax highlighting, wrapping, line numbers,
    etc.
    """

    BINDINGS = [
        Binding("escape", "focus_text_area", "Focus text area", show=False),
    ]

    @dataclass
    class LanguageChanged(Message):
        language: str | None
        footer: "TextAreaFooter"

        @property
        def control(self) -> "TextAreaFooter":
            return self.footer

    @dataclass
    class SoftWrapChanged(Message):
        value: bool
        footer: "TextAreaFooter"

        @property
        def control(self) -> "TextAreaFooter":
            return self.footer

    language: Reactive[str | None] = reactive("json", init=False)
    soft_wrap: Reactive[bool] = reactive(True, init=False)
    visual_mode: Reactive[bool] = reactive(False, init=False)
    read_only: Reactive[bool] = reactive(False, init=False)
    selection: Reactive[Selection] = reactive(Selection.cursor((0, 0)), init=False)

    def __init__(
        self,
        text_area: TextArea,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(name=name, id=id, classes=classes, disabled=disabled)
        self.text_area = text_area
        self.set_reactive(TextAreaFooter.read_only, text_area.read_only)
        self.set_reactive(TextAreaFooter.language, text_area.language)
        self.set_reactive(TextAreaFooter.soft_wrap, text_area.soft_wrap)
        self.set_reactive(TextAreaFooter.selection, text_area.selection)
        if isinstance(text_area, ReadOnlyTextArea):
            self.set_reactive(TextAreaFooter.visual_mode, text_area.visual_mode)

    def watch_selection(self, selection: Selection) -> None:
        row, column = selection.end
        self.cursor_location_label.update(f"{row + 1}:{column + 1}")

    def watch_visual_mode(self, value: bool) -> None:
        label = self.query_one("#mode-label", Label)
        label.set_class(value, "visual-mode")
        label.update("Visual" if value else "")

    def watch_read_only(self, value: bool) -> None:
        label = self.query_one("#rw-label", Label)
        label.set_class(value, "read-only")
        label.update("read-only" if value else "")

    def compose(self) -> ComposeResult:
        yield Label("", id="mode-label")
        with Horizontal(classes="dock-right w-auto"):
            yield Label("1:1", id="location-label")
            read_only = "read-only" if self.read_only else ""
            yield Label(read_only, id="rw-label", classes=read_only)
            yield PostingSelect(
                prompt="Content type",
                value=self.language,
                allow_blank=False,
                options=[("JSON", "json"), ("HTML", "html"), ("Text", None)],
                id="content-type-select",
            ).data_bind(value=TextAreaFooter.language)
            yield Checkbox(
                label="Wrap",
                value=self.soft_wrap,
                button_first=False,
                id="wrap-checkbox",
            ).data_bind(value=TextAreaFooter.soft_wrap)

    @on(Select.Changed, selector="#content-type-select")
    def update_language(self, event: Select.Changed) -> None:
        # The footer updates itself when the language changes,
        # but then broadcasts this change up to anyone who cares.
        event.stop()
        self.language = event.value
        self.post_message(self.LanguageChanged(self.language, self))

    @on(Checkbox.Changed, selector="#wrap-checkbox")
    def update_soft_wrap(self, event: Checkbox.Changed) -> None:
        event.stop()
        self.soft_wrap = event.value
        self.post_message(self.SoftWrapChanged(self.soft_wrap, self))

    @property
    def cursor_location_label(self) -> Label:
        return self.query_one("#location-label", Label)

    def action_focus_text_area(self) -> None:
        self.text_area.focus()


class PostingTextArea(TextArea):
    BINDINGS = [
        Binding("f3,ctrl+P", "open_in_pager", "Pager", id="open-in-pager"),
        Binding("f4,ctrl+E", "open_in_editor", "Editor", id="open-in-editor"),
    ]

    OPENING_BRACKETS = {
        "(": ")",
        "[": "]",
        "{": "}",
    }

    CLOSING_BRACKETS = {v: k for k, v in OPENING_BRACKETS.items()}

    def on_mount(self) -> None:
        self.indent_width = 2
        self.cursor_blink = SETTINGS.get().text_input.blinking_cursor

        self.register_theme(POSTING_THEME)
        self.register_theme(MONOKAI_THEME)
        self.register_theme(GITHUB_LIGHT_THEME)
        self.register_theme(DRACULA_THEME)

        empty = len(self.text) == 0
        self.set_class(empty, "empty")

        self.on_theme_change(self.app.current_theme)
        self.app.theme_changed_signal.subscribe(self, self.on_theme_change)

    def on_theme_change(self, theme: TextualTheme) -> None:
        builtin_theme = theme.variables.get("syntax-theme")
        if isinstance(builtin_theme, str):
            # A builtin theme was requested
            try:
                self.theme = builtin_theme
            except ThemeDoesNotExist:
                self.app.exit(
                    return_code=GENERAL_ERROR,
                    message=f"The syntax theme {builtin_theme!r} is invalid.",
                )
        else:
            # Generate a TextAreaTheme from the Textual them
            text_area_theme = Theme.text_area_theme_from_theme_variables(
                self.app.theme_variables
            )
            self.register_theme(text_area_theme)
            self.theme = text_area_theme.name

        self.call_after_refresh(self.refresh)

    @on(TextArea.Changed)
    def on_change(self, event: TextArea.Changed) -> None:
        empty = len(self.text) == 0
        self.set_class(empty, "empty")

    def action_open_in_editor(self) -> None:
        editor_command = SETTINGS.get().editor
        if not editor_command:
            self.app.notify(
                severity="warning",
                title="No editor configured",
                message="Set the [b]$EDITOR[/b] environment variable.",
            )
            return

        self._open_as_tempfile(editor_command)

    def action_open_in_pager(self) -> None:
        settings = SETTINGS.get()

        # If the language is JSON and the user has declared they
        # want to use a specific pager for JSON, let's use that.
        if self.language == "json" and settings.pager_json:
            pager_command = settings.pager_json
            if not pager_command:
                self.app.notify(
                    severity="warning",
                    title="No JSON pager configured",
                    message="Set the [b]$POSTING_PAGER_JSON[/b] environment variable.",
                )
                return
        else:
            pager_command = settings.pager
            if not pager_command:
                self.app.notify(
                    severity="warning",
                    title="No pager configured",
                    message="Set the [b]$POSTING_PAGER[/b] environment variable.",
                )
                return

        self._open_as_tempfile(pager_command)

    def _open_as_tempfile(self, command: str) -> None:
        editor_args: list[str] = shlex.split(command)

        if self.language in {"json", "html", "yaml"}:
            suffix = f".{self.language}"
        elif self.language == "python":
            suffix = ".py"
        else:
            suffix = ""

        with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as temp_file:
            temp_file_name = temp_file.name
            temp_file.write(self.text.encode("utf-8"))
            temp_file.flush()

        editor_args.append(temp_file_name)

        with self.app.suspend():
            try:
                subprocess.call(editor_args)
            except OSError:
                self.app.notify(
                    severity="error",
                    title="Can't run command",
                    message=f"The command [b]{command}[/b] failed to run.",
                )

        with open(temp_file_name, "r", encoding="utf-8") as temp_file:
            if not self.read_only:
                self.text = temp_file.read()

        os.remove(temp_file_name)
        self.app.refresh()

    def on_key(self, event: events.Key) -> None:
        character = event.character

        if self.read_only:
            return

        if character in self.OPENING_BRACKETS:
            opener = character
            closer = self.OPENING_BRACKETS[opener]
            self.insert(f"{opener}{closer}")
            self.move_cursor_relative(columns=-1)
            event.prevent_default()
        elif character in self.CLOSING_BRACKETS:
            # If we're currently at a closing bracket and
            # we type the same closing bracket, move the cursor
            # instead of inserting a character.
            if self._matching_bracket_location:
                row, col = self.cursor_location
                line = self.document.get_line(row)
                if character == line[col]:
                    event.prevent_default()
                    self.move_cursor_relative(columns=1)
        elif event.key == "enter":
            row, column = self.cursor_location
            line = self.document.get_line(row)
            if not line:
                return

            column = min(column, len(line) - 1)
            character_locations = self._yield_character_locations_reverse(
                (row, max(0, column - 1))
            )
            rstrip_line = line[: column + 1].rstrip()
            anchor_char = rstrip_line[-1] if rstrip_line else None
            get_content_start_column = self.get_content_start_column
            get_column_width = self.get_column_width
            try:
                for character, _location in character_locations:
                    # Ignore whitespace
                    if character.isspace():
                        continue
                    elif character in self.OPENING_BRACKETS:
                        # We found an opening bracket on this line,
                        # so check the indentation of the line.
                        # The newly created line should have increased
                        # indentation.
                        content_start_col = get_content_start_column(line)
                        width = get_column_width(row, content_start_col)
                        width_to_indent = max(
                            width + self.indent_width, self.indent_width
                        )

                        target_location = row + 1, column + width_to_indent
                        insert_text = "\n" + " " * width_to_indent
                        if anchor_char in self.CLOSING_BRACKETS:
                            # If there's a bracket under the cursor, we should
                            # ensure that gets indented too.
                            insert_text += "\n" + " " * content_start_col

                        self.insert(insert_text)
                        self.cursor_location = target_location
                        event.prevent_default()
                        break
                    else:
                        content_start_col = get_content_start_column(line)
                        width = get_column_width(row, content_start_col)
                        self.insert("\n" + " " * width)
                        event.prevent_default()
                        break
            except IndexError:
                return

        self._restart_blink()

    def get_content_start_column(self, line: str) -> int:
        content_start_col = 0
        for index, char in enumerate(line):
            if not char.isspace():
                content_start_col = index
                break
        return content_start_col

    def _yield_character_locations_reverse(
        self, start: Location
    ) -> Iterable[tuple[str, Location]]:
        row, column = start
        document = self.document
        line_count = document.line_count

        while line_count > row >= 0:
            line = document[row]
            if column == -1:
                column = len(line) - 1
            while column >= 0:
                yield line[column], (row, column)
                column -= 1
            row -= 1


BRACKETS = set("()[]{}")


class ReadOnlyTextArea(PostingTextArea):
    """
    A read-only text area.
    """

    BINDINGS = [
        Binding("up,k", "cursor_up", "Cursor Up", show=False),
        Binding("down,j", "cursor_down", "Cursor Down", show=False),
        Binding("right,l", "cursor_right", "Cursor Right", show=False),
        Binding("left,h", "cursor_left", "Cursor Left", show=False),
        Binding("shift+up,K", "cursor_up(True)", "cursor up select", show=False),
        Binding("shift+down,J", "cursor_down(True)", "cursor down select", show=False),
        Binding("shift+left,H", "cursor_left(True)", "cursor left select", show=False),
        Binding(
            "shift+right,L", "cursor_right(True)", "cursor right select", show=False
        ),
        Binding("ctrl+left,b", "cursor_word_left", "cursor word left", show=False),
        Binding("ctrl+right,w", "cursor_word_right", "cursor word right", show=False),
        Binding(
            "home,ctrl+a,0,^", "cursor_line_start", "cursor line start", show=False
        ),
        Binding("end,ctrl+e,$", "cursor_line_end", "cursor line end", show=False),
        Binding("pageup,ctrl+b", "cursor_page_up", "cursor page up", show=False),
        Binding("pagedown,ctrl+f", "cursor_page_down", "cursor page down", show=False),
        Binding("ctrl+d", "cursor_half_page_down", "cursor half page down", show=False),
        Binding("ctrl+u", "cursor_half_page_up", "cursor half page up", show=False),
        Binding(
            "ctrl+shift+left,B",
            "cursor_word_left(True)",
            "cursor left word select",
            show=False,
        ),
        Binding(
            "ctrl+shift+right,W",
            "cursor_word_right(True)",
            "cursor right word select",
            show=False,
        ),
        Binding("f6,V", "select_line", "select line", show=False),
        Binding(
            "v",
            "toggle_visual_mode",
            description="Toggle visual mode",
            show=False,
        ),
        Binding(
            "y,c",
            "copy_to_clipboard",
            description="Copy selection",
            show=False,
        ),
        Binding("g", "cursor_top", "Go to top", show=False),
        Binding("G", "cursor_bottom", "Go to bottom", show=False),
        Binding("%", "cursor_to_matched_bracket", "Go to matched bracket", show=False),
    ]

    def __init__(
        self,
        text: str = "",
        *,
        language: str | None = None,
        theme: str = "css",
        soft_wrap: bool = True,
        tab_behavior: Literal["focus"] | Literal["indent"] = "focus",
        read_only: bool = True,
        show_line_numbers: bool = False,
        max_checkpoints: int = 50,
        select_on_focus: bool = False,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(
            text,
            language=language,
            theme=theme,
            soft_wrap=soft_wrap,
            tab_behavior=tab_behavior,
            read_only=read_only,
            show_line_numbers=show_line_numbers,
            max_checkpoints=max_checkpoints,
            name=name,
            id=id,
            classes=classes,
            disabled=disabled,
        )
        self.select_on_focus = select_on_focus

    @dataclass
    class VisualModeToggled(Message):
        value: bool
        text_area: TextArea

        @property
        def control(self) -> TextArea:
            return self.text_area

    visual_mode = reactive(False, init=False)

    def action_toggle_visual_mode(self):
        self.visual_mode = not self.visual_mode

    def watch_visual_mode(self, value: bool) -> None:
        self.cursor_blink = SETTINGS.get().text_input.blinking_cursor and not value
        if not value:
            self.selection = Selection.cursor(self.selection.end)

        self.set_class(value, "visual-mode")
        self.post_message(self.VisualModeToggled(value, self))

    def action_cursor_up(self, select: bool = False) -> None:
        return super().action_cursor_up(self.visual_mode or select)

    def action_cursor_right(self, select: bool = False) -> None:
        return super().action_cursor_right(self.visual_mode or select)

    def action_cursor_down(self, select: bool = False) -> None:
        return super().action_cursor_down(self.visual_mode or select)

    def action_cursor_left(self, select: bool = False) -> None:
        return super().action_cursor_left(self.visual_mode or select)

    def action_cursor_line_end(self, select: bool = False) -> None:
        return super().action_cursor_line_end(self.visual_mode or select)

    def action_cursor_line_start(self, select: bool = False) -> None:
        return super().action_cursor_line_start(self.visual_mode or select)

    def action_cursor_word_left(self, select: bool = False) -> None:
        return super().action_cursor_word_left(self.visual_mode or select)

    def action_cursor_word_right(self, select: bool = False) -> None:
        return super().action_cursor_word_right(self.visual_mode or select)

    def action_copy_to_clipboard(self) -> None:
        text_to_copy = self.selected_text
        if text_to_copy:
            message = f"Copied {len(text_to_copy)} characters."
            title = "Selection copied"
        else:
            text_to_copy = self.text
            message = f"Copied ({len(text_to_copy)} characters)."
            title = "Text copied"

        try:
            import pyperclip

            pyperclip.copy(text_to_copy)
        except pyperclip.PyperclipException as exc:
            self.notify(
                str(exc),
                title="Clipboard error",
                severity="error",
                timeout=10,
            )
        else:
            self.notify(message, title=title)

        self.visual_mode = False

    def action_cursor_top(self) -> None:
        self.selection = Selection.cursor((0, 0))

    def action_cursor_bottom(self) -> None:
        self.selection = Selection.cursor((self.document.line_count - 1, 0))

    def action_cursor_to_matched_bracket(self) -> None:
        # If we're already on a bracket which has a match, just jump to it and return.
        matching_bracket_location = self.matching_bracket_location
        if matching_bracket_location:
            self.selection = Selection.cursor(matching_bracket_location)
            return

        # Look for a bracket on the rest of the cursor line.
        # If we find a bracket, jump to its match.
        row, column = self.selection.end
        rest_of_line = self.document.get_line(row)[column:]
        for column_index, char in enumerate(rest_of_line, start=column):
            if char in BRACKETS:
                matched_bracket = self.find_matching_bracket(char, (row, column_index))
                if matched_bracket:
                    self.selection = Selection.cursor(matched_bracket)
                    break

    def action_cursor_half_page_down(self) -> None:
        """Move the cursor and scroll down half of a page."""
        half_height = self.content_size.height // 2
        _, cursor_location = self.selection
        target = self.navigator.get_location_at_y_offset(
            cursor_location,
            half_height,
        )
        self.scroll_relative(y=half_height, animate=False)
        self.move_cursor(target)

    def action_cursor_half_page_up(self) -> None:
        """Move the cursor and scroll down half of a page."""
        half_height = self.content_size.height // 2
        _, cursor_location = self.selection
        target = self.navigator.get_location_at_y_offset(
            cursor_location,
            -half_height,
        )
        self.scroll_relative(y=-half_height, animate=False)
        self.move_cursor(target)

    def on_focus(self) -> None:
        if self.select_on_focus:
            self.select_all()

    def get_content_start_column(self, line: str) -> int:
        content_start_col = 0
        for index, char in enumerate(line):
            if not char.isspace():
                content_start_col = index
                break
        return content_start_col

    def _yield_character_locations_reverse(
        self, start: Location
    ) -> Iterable[tuple[str, Location]]:
        row, column = start
        document = self.document
        line_count = document.line_count

        while line_count > row >= 0:
            line = document[row]
            if column == -1:
                column = len(line) - 1
            while column >= 0:
                yield line[column], (row, column)
                column -= 1
            row -= 1


class TextEditor(Vertical):
    soft_wrap: Reactive[bool] = reactive(True, init=False)
    language: Reactive[str | None] = reactive("json", init=False)
    read_only: Reactive[bool] = reactive(False, init=False)

    def __init__(
        self,
        text_area: TextArea,
        footer: TextAreaFooter,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(name=name, id=id, classes=classes, disabled=disabled)
        self.text_area = text_area
        self.footer = footer
        self.read_only = text_area.read_only

    def compose(self) -> ComposeResult:
        yield self.text_area.data_bind(
            TextEditor.soft_wrap,
            TextEditor.language,
            TextEditor.read_only,
        )
        yield self.footer.data_bind(
            TextEditor.soft_wrap,
            TextEditor.language,
            TextEditor.read_only,
        )

    @on(TextArea.SelectionChanged)
    def update_selection(self, event: TextArea.SelectionChanged) -> None:
        self.footer.selection = event.selection

    @on(ReadOnlyTextArea.VisualModeToggled)
    def update_visual_mode(self, event: ReadOnlyTextArea.VisualModeToggled) -> None:
        self.footer.visual_mode = event.value

    @on(TextAreaFooter.LanguageChanged, selector="TextAreaFooter")
    def update_language(self, event: TextAreaFooter.LanguageChanged) -> None:
        self.language = event.language

    @on(TextAreaFooter.SoftWrapChanged, selector="TextAreaFooter")
    def update_soft_wrap(self, event: TextAreaFooter.SoftWrapChanged) -> None:
        self.soft_wrap = event.value

    @property
    def text(self) -> str:
        return self.text_area.text

    @property
    def content_type(self) -> str | None:
        """Return the content type associated with the current language."""
        if self.language == "json":
            return "application/json"
        elif self.language == "html":
            return "text/html"
        else:
            return None


VSCODE = TextAreaTheme.get_builtin_theme("vscode_dark")
POSTING_THEME = TextAreaTheme(
    name="posting",
    syntax_styles={
        # "json.error": Style.parse("u #dc2626"),
        **(VSCODE.syntax_styles if VSCODE else {}),
    },
)
MONOKAI = TextAreaTheme.get_builtin_theme("monokai")
MONOKAI_THEME = TextAreaTheme(
    name="monokai",
    syntax_styles={
        # "json.error": Style.parse("u #dc2626"),
        **(MONOKAI.syntax_styles if MONOKAI else {}),
    },
)

GITHUB_LIGHT = TextAreaTheme.get_builtin_theme("github_light")
GITHUB_LIGHT_THEME = TextAreaTheme(
    name="github_light",
    syntax_styles={
        # "json.error": Style.parse("u #dc2626"),
        **(GITHUB_LIGHT.syntax_styles if GITHUB_LIGHT else {}),
    },
)

DRACULA = TextAreaTheme.get_builtin_theme("dracula")
DRACULA_THEME = TextAreaTheme(
    name="dracula",
    syntax_styles={
        # "json.error": Style.parse("u #dc2626"),
        **(DRACULA.syntax_styles if DRACULA else {}),
    },
)
