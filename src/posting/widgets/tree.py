from typing import Generator, TypeVar
from textual.binding import Binding
from textual.widgets import Tree
from textual.widgets.tree import TreeNode

T = TypeVar("T")


class PostingTree(Tree[T]):
    DEFAULT_CSS = """\
    PostingTree { 
        scrollbar-size-horizontal: 0;
        & .node-selected {
            background: $primary-lighten-1;
            color: $text;
            text-style: bold;
        }
    }
    
    """

    BINDINGS = [
        Binding("k", "cursor_up", "Cursor Up", show=False),
        Binding("j", "cursor_down", "Cursor Down", show=False),
        Binding("K", "cursor_up_parent", "Cursor Up Parent", show=False),
        Binding("J", "cursor_down_parent", "Cursor Down Parent", show=False),
        Binding("g", "scroll_home", "Cursor To Top", show=False),
        Binding("G", "scroll_end", "Cursor To Bottom", show=False),
        Binding("enter,l,h", "select_cursor", "Select Cursor", show=False),
        Binding("space,r", "toggle_node", "Toggle Expand", show=False),
    ]

    def action_cursor_up_parent(self) -> None:
        """Move the cursor to the previous collapsible node."""
        start_line = max(self.cursor_line - 1, 0)
        for line in range(start_line, -1, -1):
            node = self.get_node_at_line(line)
            if node and node.allow_expand:
                self.cursor_line = line
                return

    def action_cursor_down_parent(self) -> None:
        """Move the cursor to the next collapsible node."""
        max_index = len(self._tree_lines) - 1
        start_line = min(self.cursor_line + 1, max_index)
        for line in range(start_line, max_index + 1):
            node = self.get_node_at_line(line)
            if node and node.allow_expand:
                self.cursor_line = line
                return

    def walk_nodes(self) -> Generator[TreeNode[T], None, None]:
        """Walk the nodes of the tree."""
        for node in self._tree_nodes.values():
            yield node
