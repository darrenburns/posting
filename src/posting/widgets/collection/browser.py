from typing import Union
from rich.style import Style
from rich.text import Text
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical, VerticalScroll
from textual.reactive import Reactive, reactive
from textual.widgets import Static, Tree
from textual.widgets.tree import TreeNode

from posting.collection import Collection, RequestModel


TOGGLE_STYLE = Style.from_meta({"toggle": True}) + Style(dim=True)
SUFFIX = ".posting.yaml"


CollectionNode = Union[Collection, RequestModel]


class CollectionTree(Tree[CollectionNode]):
    BINDINGS = [
        Binding("k", "cursor_up", "Cursor Up", show=False),
        Binding("j", "cursor_down", "Cursor Down", show=False),
        Binding("enter,l,h", "select_cursor", "Select Cursor", show=False),
        Binding("space,r", "toggle_node", "Toggle Expand", show=False),
    ]

    def render_label(
        self, node: TreeNode[CollectionNode], base_style: Style, style: Style
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
        if node_label.plain.endswith(SUFFIX):
            node_label = Text(node_label.plain[: -len(SUFFIX)], style=node_label.style)

        if not self.is_mounted:
            return node_label

        if node._allow_expand:
            prefix = (
                "▼ " if node.is_expanded else "▶ ",
                base_style + TOGGLE_STYLE,
            )
            node_label.append("/")
            node_label.stylize(Style(dim=True, bold=True))
        else:
            prefix = (" ", base_style)

        node_label.stylize(style)
        text = Text.assemble(prefix, node_label)
        return text


class RequestPreview(VerticalScroll):
    DEFAULT_CSS = """\
        RequestPreview {
            height: auto;
            max-height: 50%;
            padding: 0 1;
            dock: bottom;
            background: transparent;
            border-top: solid $accent 50%;
            &.hidden {
                display: none;
            }
        }
    """

    request: Reactive[RequestModel | None] = reactive(None)

    def compose(self) -> ComposeResult:
        self.can_focus = False
        yield Static("", id="description")

    def watch_request(self, request: RequestModel | None) -> None:
        self.set_class(request is None, "hidden")
        if request:
            description = self.query_one("#description", Static)
            description.update(request.description)


class CollectionBrowser(Vertical):
    DEFAULT_CSS = """\
    CollectionBrowser {
        height: 1fr;
        dock: left;
        width: auto;
        max-width: 42;
        & Tree {
            min-width: 20;
            background: transparent;
        }
    }
    """

    def __init__(
        self,
        collection: Collection | None = None,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(name=name, id=id, classes=classes, disabled=disabled)
        self.collection = collection

    def compose(self) -> ComposeResult:
        self.border_title = "Collection"
        self.add_class("section")
        collection = self.collection
        if collection is None:
            return

        tree = CollectionTree(label=collection.name, data=collection)
        tree.guide_depth = 1
        tree.show_root = False
        tree.show_guides = False

        def add_collection_to_tree(
            parent_node: TreeNode[CollectionNode], collection: Collection
        ) -> None:
            # Add the requests (leaf nodes)
            for request in collection.requests:
                parent_node.add_leaf(request.name, data=request)

            # Add the subcollections (child nodes)
            for child_collection in collection.children:
                child_node = parent_node.add(
                    child_collection.name, data=child_collection
                )
                add_collection_to_tree(child_node, child_collection)

        # Start building the tree from the root node
        add_collection_to_tree(tree.root, collection)

        tree.root.expand_all()
        yield tree
        yield RequestPreview()

    # TODO - implement the node selected event
    # @on(Tree.NodeSelected)
    # def on_node_selected(self, event: Tree.NodeSelected[CollectionNode]) -> None:
    #     print(event.node.data)

    @on(Tree.NodeHighlighted)
    def on_node_highlighted(self, event: Tree.NodeHighlighted[CollectionNode]) -> None:
        node_data = event.node.data
        # TODO - display more preview data.
        #  It's already all in the node, just need to display it.
        if isinstance(node_data, RequestModel):
            self.request_preview.request = node_data
        else:
            self.request_preview.request = None

    @property
    def request_preview(self) -> RequestPreview:
        return self.query_one(RequestPreview)
