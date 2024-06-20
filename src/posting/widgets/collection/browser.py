from dataclasses import dataclass
from functools import partial
import os
from pathlib import Path
from typing import Union
from urllib.parse import urlparse
from rich.style import Style
from rich.text import Text, TextType
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical, VerticalScroll
from textual.geometry import Region
from textual.message import Message
from textual.reactive import Reactive, reactive
from textual.widgets import Static, Tree
from textual.widgets.tree import TreeNode

from posting.collection import Collection, RequestModel
from posting.widgets.collection.new_request_modal import (
    NewRequestData,
    NewRequestModal,
)


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

    COMPONENT_CLASSES = {
        "node-selected",
    }

    DEFAULT_CSS = """\
    CollectionTree { 
        scrollbar-size-horizontal: 0;
        & .node-selected {
            background: $primary-lighten-1;
            color: $text;
            text-style: bold;
        }
    }
    
    """

    def __init__(
        self,
        label: TextType,
        data: CollectionNode | None = None,
        *,
        name: str | None = None,
        id: str | None = None,
        classes: str | None = None,
        disabled: bool = False,
    ) -> None:
        super().__init__(
            label,
            data,
            name=name,
            id=id,
            classes=classes,
            disabled=disabled,
        )
        self.cached_base_urls: set[str] = set()

    @dataclass
    class RequestSelected(Message):
        request: RequestModel
        node: TreeNode[CollectionNode]
        tree: "CollectionTree"

        @property
        def control(self) -> "CollectionTree":
            return self.tree

    @dataclass
    class RequestCacheUpdated(Message):
        cached_base_urls: list[str]
        tree: "CollectionTree"

        @property
        def control(self) -> "CollectionTree":
            return self.tree

    currently_open: Reactive[TreeNode[CollectionNode] | None] = reactive(None)

    def key_p(self):
        print(
            repr(self.cursor_node.data if self.cursor_node is not None else "no node")
        )

    def watch_currently_open(self, node: TreeNode[CollectionNode] | None) -> None:
        if node and isinstance(node.data, RequestModel):
            self.post_message(
                self.RequestSelected(
                    request=node.data,
                    node=node,
                    tree=self,
                )
            )

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
            method = (
                f"{'█ ' if node is self.currently_open else ' '}{node.data.method[:3]} "
                if isinstance(node.data, RequestModel)
                else ""
            )
            node_label = Text.assemble((method, Style(dim=True)), node_label)
            prefix = ""

        node_label.stylize(style)

        if node is self.currently_open:
            open_style = self.get_component_rich_style("node-selected")
        else:
            open_style = ""

        text = Text.assemble(
            prefix,
            node_label,
            style=open_style,
        )
        return text

    def scroll_to_line(self, line: int, animate: bool = False) -> None:
        """Scroll to the given line.

        Overridden because I don't like the default horizontal scrolling behavior.

        Args:
            line: A line number.
            animate: Enable animation.
        """
        region = Region(0, line, 1, 1)
        self.scroll_to_region(region, animate=animate, force=True)

    def on_mount(self) -> None:
        self.post_message(
            self.RequestCacheUpdated(
                cached_base_urls=list(self.cached_base_urls),
                tree=self,
            )
        )

    @on(Tree.NodeSelected)
    def on_node_selected(self, event: Tree.NodeSelected[CollectionNode]) -> None:
        event.stop()
        if isinstance(event.node.data, RequestModel):
            self.currently_open = event.node
            self._clear_line_cache()
            self.refresh()

    async def new_request_flow(self, initial_request: RequestModel | None) -> None:
        """Start the flow to create a new request.

        Args:
            initial_request: If the user has already started filling out request info
            in the UI, then we can pre-fill the modal with that info.
        """
        # Determine where in the tree the new request will live.
        # This is based on the current cursor location within the tree.
        cursor_node = self.cursor_node
        if cursor_node is None:
            parent_node = self.root
        else:
            node_data = cursor_node.data
            if isinstance(node_data, Collection):
                parent_node = cursor_node
            elif isinstance(node_data, RequestModel):
                parent_node = cursor_node.parent or self.root
            else:
                parent_node = self.root

        root_path = self.root.data.path
        assert root_path is not None, "root should have a path"

        def _handle_new_request_data(new_request_data: NewRequestData | None) -> None:
            """Get the new request data from the modal, and update the UI with it."""
            if new_request_data is None:
                # Happens when the user presses `escape` while in the modal.
                return

            # The user confirms the details in the modal, so use these details
            # to create a new RequestModel and add it to the tree.
            request_name = new_request_data.title
            request_description = new_request_data.description
            request_directory = new_request_data.directory

            if request_directory.strip() == "":
                request_directory = "."

            file_name = new_request_data.file_name
            final_path = root_path / request_directory / f"{file_name}"

            if initial_request is not None:
                # Ensure that any data which was filled by the user in the UI is included in the
                # node data, alongside the typed title and filename from the modal.
                new_request = initial_request.model_copy(
                    update={
                        "name": request_name,
                        "description": request_description,
                        "path": final_path,
                    }
                )
            else:
                # We're creating an entirely new request from the sidebar
                new_request = RequestModel(
                    name=request_name,
                    path=final_path,
                    description=request_description,
                )

            # Traverse the path, creating any intermediate collections
            # which are not already in the tree.
            path_parts = request_directory.strip(os.path.sep).split(os.path.sep)
            pointer = self.root
            subpath = self.root.data.path
            for part in path_parts:
                if part == ".":
                    continue

                subpath = os.path.join(subpath, part)

                found = False
                for child in pointer.children:
                    if isinstance(child.data, Collection) and child.data.name == part:
                        pointer = child
                        found = True
                        break

                if not found:
                    # Collection couldn't be found at this level of the tree.
                    # Create it and move down.
                    new_collection = Collection(name=part, path=Path(subpath))
                    pointer = pointer.add(part, data=new_collection)
                    pointer.expand()

            # Attach to the relevant node
            new_node = self.add_request(
                new_request, parent_node if pointer is self.root else pointer
            )
            self.currently_open = new_node

            # Persist the request on disk.
            save_path = new_request.path
            assert save_path is not None, "new request must have a path"
            new_request.save_to_disk(save_path)
            self.notify(
                title="Request saved",
                message=f"{save_path.resolve().relative_to(root_path.resolve())}",
                timeout=3,
            )

            def post_new_request() -> None:
                self.select_node(new_node)
                self.scroll_to_node(new_node, animate=False)

            self.call_after_refresh(post_new_request)

        parent_path = parent_node.data.path
        assert parent_path is not None, "parent should have a path"
        await self.app.push_screen(
            NewRequestModal(
                initial_directory=str(
                    parent_path.resolve().relative_to(root_path.resolve())
                ),
                initial_title="" if initial_request is None else initial_request.name,
                initial_description=""
                if initial_request is None
                else initial_request.description,
            ),
            callback=_handle_new_request_data,
        )

    def add_request(
        self, request: RequestModel, parent_node: TreeNode[CollectionNode]
    ) -> TreeNode[CollectionNode]:
        """Add a new request to the tree, and cache data from it."""
        self.cache_request(request)
        return parent_node.add_leaf(request.name, data=request)

    def cache_request(self, request: RequestModel) -> None:
        def get_base_url(url: str) -> str | None:
            try:
                parsed_url = urlparse(url)
                # Check if the scheme and netloc are present
                if parsed_url.scheme and parsed_url.netloc:
                    base_url = f"{parsed_url.scheme}://{parsed_url.netloc}"
                    return base_url
            except Exception:
                return None

        base_url = get_base_url(request.url)
        if base_url:
            self.cached_base_urls.add(base_url)
            # Post a message up to the screen so that it can inform
            # the URL bar that the autocomplete suggestions have changed.
            self.post_message(
                self.RequestCacheUpdated(
                    cached_base_urls=list(self.cached_base_urls),
                    tree=self,
                )
            )


class RequestPreview(VerticalScroll):
    DEFAULT_CSS = """\
        RequestPreview {
            color: $text-muted;
            background: transparent;
            dock: bottom;
            height: auto;
            max-height: 50%;
            padding: 0 1;
            border-top: solid $accent 35%;
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
        self.set_class(request is None or not request.description, "hidden")
        if request:
            description = self.query_one("#description", Static)
            description.update(request.description)


class CollectionBrowser(Vertical):
    DEFAULT_CSS = """\
    CollectionBrowser {
        height: 1fr;
        dock: left;
        width: auto;
        max-width: 33%;
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

        tree = CollectionTree(
            label=collection.name,
            data=collection,
            id="collection-tree",
        )
        tree.guide_depth = 1
        tree.show_root = False
        tree.show_guides = False
        self.border_subtitle = collection.name

        def add_collection_to_tree(
            parent_node: TreeNode[CollectionNode], collection: Collection
        ) -> None:
            # Add the requests (leaf nodes)
            for request in collection.requests:
                tree.add_request(request, parent_node)

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

    @on(CollectionTree.RequestSelected)
    def on_request_selected(self, event: CollectionTree.RequestSelected) -> None:
        if isinstance(event.node.data, RequestModel):
            self.request_preview.request = event.node.data

    @on(Tree.NodeHighlighted)
    def on_node_highlighted(self, event: Tree.NodeHighlighted[CollectionNode]) -> None:
        node_data = event.node.data
        # TODO - display more preview data.
        #  It's already all in the node, just need to display it.
        if isinstance(node_data, RequestModel):
            self.request_preview.request = node_data
        else:
            self.request_preview.request = None

        self.set_timer(
            0.05,
            partial(self.collection_tree.scroll_to_node, event.node, animate=False),
        )

    def update_currently_open_node(self, request_model: RequestModel) -> None:
        """Update the request tree node with the new request model."""
        currently_open = self.collection_tree.currently_open
        if currently_open is not None and isinstance(currently_open.data, RequestModel):
            currently_open.data = request_model
            currently_open.set_label(request_model.name or "")
            self.collection_tree.cache_request(request_model)
            currently_open.refresh()
            # Update the description preview if it's the one currently being displayed.
            if currently_open is self.collection_tree.cursor_node:
                self.request_preview.request = request_model

    @property
    def request_preview(self) -> RequestPreview:
        return self.query_one(RequestPreview)

    @property
    def collection_tree(self) -> CollectionTree:
        return self.query_one(CollectionTree)
