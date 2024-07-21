from posting.help_screen import HelpData
from posting.highlighters import VariableHighlighter
from posting.widgets.input import PostingInput

from posting.widgets.variable_autocomplete import VariableAutoComplete


class VariableInput(PostingInput):
    help = HelpData(
        title="Variable-Aware Input",
        description="""\
An input field which supports auto-completion and highlighting of variables
from the environment (loaded via `--env`).
Use the `up` and `down` keys to navigate the dropdown list when it's visible.
Press `enter` to insert a dropdown item.
Press `tab` to both insert *and* shift focus.
""",
    )

    def on_mount(self) -> None:
        self.highlighter = VariableHighlighter()
        self.auto_complete = VariableAutoComplete(
            candidates=[],
            target=self,
        )
        self.screen.mount(self.auto_complete)
