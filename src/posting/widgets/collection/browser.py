import bisect
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
from posting.config import SETTINGS
from posting.files import get_unique_request_filename
from posting.help_screen import HelpData
from posting.save_request import generate_request_filename
from posting.widgets.collection.new_request_modal import (
    NewRequestData,
    NewRequestModal,
)
from posting.widgets.confirmation import ConfirmationModal
from posting.widgets.tree import PostingTree


TOGGLE_STYLE = Style.from_meta({"toggle": True}) + Style(dim=True)
SUFFIX = ".posting.yaml"


CollectionNode = Union[Collection, RequestModel]


class CollectionTree(PostingTree[CollectionNode]):
    help = HelpData(
        title="Collection Browser",
        description="""\
Shows all `*.posting.yaml` request files resolved from the specified collection directory.
- Press `ctrl+n` to create a new request at the current cursor location.
- Press `d` to duplicate the request under the cursor (showing the request info modal so you can edit title/description/directory).
- Press `D` (`shift`+`d`) to quickly duplicate the request under the cursor (without showing the request info modal).
- `j` and `k` can be used to navigate the tree.
- `J` and `K` (shift+j and shift+k) jumps between sub-collections.
- `g` and `G` jumps to the top and bottom of the tree, respectively.
- `backspace` deletes the request under the cursor.
- `shift+backspace` deletes the request under the cursor, skipping the confirmation dialog.
Sub-collections cannot be deleted from the UI yet.
""",
    )

    BINDING_GROUP_TITLE = "Collection Browser"

    BINDINGS = [
        Binding(
            "d",
            "duplicate_request",
            "Dupe",
            tooltip="Duplicate the request under the cursor and show the 'New Request' modal to change the name/description.",
        ),
        Binding(
            "D",
            "quick_duplicate_request",
            "Quick Dupe",
            show=False,
            tooltip="Duplicate the request and automatically assign a unique name.",
        ),
        Binding(
            "backspace",
            "delete_request_with_confirmation",
            "Delete",
            tooltip="Delete the request under the cursor.",
        ),
        Binding(
            "shift+backspace",
            "delete_request",
            "Delete",
            show=False,
            tooltip="Delete the collection under the cursor.",
        ),
    ]

    COMPONENT_CLASSES = {
        "node-selected",
    }

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
    class RequestAdded(Message):
        request: RequestModel
        node: TreeNode[CollectionNode]
        tree: "CollectionTree"

        @property
        def control(self) -> "CollectionTree":
            return self.tree

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
            if self._cursor_node is not node:
                node_label.stylize(Style(dim=True, bold=True))
        else:
            theme_vars = self.app.theme_variables
            default_styles = {
                "get": theme_vars.get("text-primary"),
                "post": theme_vars.get("text-success"),
                "put": theme_vars.get("text-warning"),
                "delete": theme_vars.get("text-error"),
                "options": theme_vars.get("text-muted"),
                "head": theme_vars.get("text-muted"),
            }

            method = node.data.method.lower()
            method_style = theme_vars.get(
                f"method-{method}",
                default_styles.get(method),
            )

            open_indicator = ">" if node is self.currently_open else " "
            method = (
                f"{node.data.method[:3]}" if isinstance(node.data, RequestModel) else ""
            )
            node_label = Text.assemble(
                open_indicator,
                Text(method, style=method_style),
                " ",
                node_label,
            )
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

    async def new_request_flow(self, templated_from: RequestModel | None) -> None:
        """Start the flow to create a new request.

        Args:
            templated_from: If the user has already started filling out request info
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

        # Take note of what was focused on this screen, so that we
        # can focus it again when the modal is closed.
        focused_before = self.screen.focused
        self.screen.set_focus(None)

        def _handle_new_request_data(new_request_data: NewRequestData | None) -> None:
            """Get the new request data from the modal, and update the UI with it."""
            if new_request_data is None:
                # Happens when the user presses `escape` while in the modal.
                self.screen.set_focus(focused_before)
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

            if templated_from is not None:
                # Ensure that any data which was filled by the user in the UI is included in the
                # node data, alongside the typed title and filename from the modal.
                new_request = templated_from.model_copy(
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

            new_request_parent = pointer
            parent_collection = new_request_parent.data
            sibling_requests: list[RequestModel] = (
                parent_collection.requests
                if isinstance(parent_collection, Collection)
                else []
            )

            # Find where to insert the new request amongst its new siblings.
            if sibling_requests:
                index = bisect.bisect_right(
                    sibling_requests,
                    new_request,
                )
                if index == len(sibling_requests):
                    before = None
                else:
                    before = index
            else:
                before = None

            if before is not None:
                sibling_requests.insert(before, new_request)
            else:
                sibling_requests.append(new_request)

            parent_collection.requests = sibling_requests
            # If the cursor and the newly added node have the same parent, then the cursor node
            # and the newly added node are siblings, so we should insert the newly added node
            # in an appropriate position relative to the cursor node.
            # Find where to insert the new request.

            # Attach to the relevant node. Note that the cursor node is not relevant here.
            # The only thing that matters is the directory path specified by the user.
            new_node = self.add_request(
                new_request,
                new_request_parent,
                before=before,
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
                self.screen.set_focus(focused_before)
                self.select_node(new_node)
                if new_node is not None:
                    self.scroll_to_node(new_node, animate=False)

            self.call_after_refresh(post_new_request)

        parent_path = parent_node.data.path if parent_node.data else None
        assert parent_path is not None, "parent should have a path"
        await self.app.push_screen(
            NewRequestModal(
                initial_directory=str(
                    parent_path.resolve().relative_to(root_path.resolve())
                ),
                initial_title="" if templated_from is None else templated_from.name,
                initial_description=""
                if templated_from is None
                else templated_from.description,
                parent_node=parent_node,
            ),
            callback=_handle_new_request_data,
        )

    def add_request(
        self,
        request: RequestModel,
        parent_node: TreeNode[CollectionNode],
        after: TreeNode[CollectionNode] | int | None = None,
        before: TreeNode[CollectionNode] | int | None = None,
    ) -> TreeNode[CollectionNode] | None:
        """Add a new request to the tree, and cache data from it."""
        self.cache_request(request)
        try:
            added_node = parent_node.add_leaf(
                request.name, data=request, after=after, before=before
            )
        except Exception as e:
            self.notify(
                title="Error adding request.",
                message=f"{e}",
                timeout=3,
            )
            return None
        else:
            self.post_message(self.RequestAdded(request, parent_node, self))
            return added_node

    async def action_duplicate_request(self) -> None:
        cursor_node = self.cursor_node
        if cursor_node is None:
            return

        current_request = cursor_node.data
        if not isinstance(current_request, RequestModel):
            return

        await self.new_request_flow(templated_from=current_request)

    def action_quick_duplicate_request(self) -> None:
        cursor_node = self.cursor_node
        if cursor_node is None:
            return

        cursor_request = cursor_node.data
        if not isinstance(cursor_request, RequestModel):
            return

        original_path = cursor_request.path if cursor_request.path is not None else None
        if not original_path:
            return

        new_name = cursor_request.name
        new_filename = generate_request_filename(new_name) + ".posting.yaml"
        new_filename = get_unique_request_filename(new_filename, original_path.parent)

        new_path = (
            (original_path.parent / new_filename)
            if original_path
            else Path(new_filename)
        )
        cursor_request_copy = cursor_request.model_copy(
            update={"name": new_name, "path": new_path}
        )
        cursor_request_copy.save_to_disk(new_path)
        self.add_request(
            request=cursor_request_copy,
            parent_node=cursor_node.parent if cursor_node.parent else self.root,
            after=cursor_node,
        )

    def action_delete_request(self) -> None:
        cursor_node = self.cursor_node
        if cursor_node is None:
            return

        if isinstance(cursor_node.data, RequestModel):
            cursor_request = cursor_node.data
            cursor_request.delete_from_disk()
            cursor_node.remove()

    async def action_delete_request_with_confirmation(self) -> None:
        cursor_node = self.cursor_node
        if cursor_node is None:
            return

        def deletion_callback(confirmed: bool | None) -> None:
            if confirmed is True:
                if cursor_node and isinstance(cursor_node.data, RequestModel):
                    cursor_request = cursor_node.data
                    cursor_request.delete_from_disk()
                    cursor_node.remove()

        if isinstance(cursor_node.data, RequestModel):
            cursor_path = cursor_node.data.path
            collection_root_path = self.root.data.path if self.root.data else None
            if not cursor_path or not collection_root_path:
                return

            if not cursor_path.is_relative_to(collection_root_path):
                path_to_display = cursor_path
            else:
                path_to_display = cursor_path.relative_to(collection_root_path)

            confirmation_message = (
                f"[b]Do you want to delete this request?[/]\n[i]{path_to_display}[/]"
            )

            await self.app.push_screen(
                ConfirmationModal(confirmation_message, auto_focus="confirm"),
                callback=deletion_callback,
            )

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
                # TODO: We should sort these cached URLs on frequency instead
                # of alphabetically. Bring the most used URLs to the top.
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
            width: 100%;
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
        self.styles.dock = SETTINGS.get().collection_browser.position
        self.border_title = "Collection"
        self.add_class("section")
        collection = self.collection

        yield Static(
            "[i]Collection is empty.[/]\n\nPress [b]ctrl+s[/b] to save the current request.\n\nPress [b]ctrl+h[/b] to toggle this panel.",
            id="empty-collection-label",
        )

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
        tree.cursor_line = 0
        yield tree
        yield RequestPreview()

    @on(CollectionTree.RequestAdded)
    def on_request_added(self, event: CollectionTree.RequestAdded) -> None:
        self.query_one("#empty-collection-label").display = (
            len(event.tree.root.children) == 0
        )

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
