from rich.style import Style
from rich.text import Text
from textual.app import ComposeResult
from textual.containers import Vertical
from textual.types import DirEntry
from textual.widgets import DirectoryTree
from textual.widgets.tree import TreeNode


TOGGLE_STYLE = Style.from_meta({"toggle": True})


class CollectionTree(DirectoryTree):
    def render_label(
        self, node: TreeNode[DirEntry], base_style: Style, style: Style
    ) -> Text:
        """Render a label for the given node.

        Args:
            node: A tree node.
            base_style: The base style of the widget.
            style: The additional style for the label.

        Returns:
            A Rich Text object containing the label.
        """
        # If the tree isn't mounted yet we can't use component classes to stylize
        # the label fully, so we return early.

        node_label = node._label.copy()
        node_label.stylize(style)

        if not self.is_mounted:
            return node_label

        if node._allow_expand:
            prefix = (
                "▼ " if node.is_expanded else "▶ ",
                base_style + TOGGLE_STYLE,
            )
        else:
            prefix = ("  ", base_style)

        text = Text.assemble(prefix, node_label)
        return text


class CollectionBrowser(Vertical):
    DEFAULT_CSS = """\
    CollectionBrowser {
        height: 1fr;
        dock: left;
        width: auto;
        & Tree {
            width: auto;
            min-width: 18;
            background: transparent;
        }
    }
    """

    def compose(self) -> ComposeResult:
        self.border_title = "Collections"
        self.add_class("section")
        tree = CollectionTree("./sample-collections/")
        tree.guide_depth = 2
        tree.show_root = False
        tree.show_guides = False
        tree.expand = True
        yield tree
