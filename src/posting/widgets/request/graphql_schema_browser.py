"""A browser for the whole GraphQL schema of an endpoint."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Literal

from rich.text import Text
from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical, VerticalScroll
from textual.screen import ModalScreen
from textual.widgets import Input, Label, Static, Tree
from textual.widgets.tree import TreeNode

from posting.graphql.schema import (
    Argument,
    Field,
    FieldMatch,
    OperationType,
    Schema,
    Type,
)
from posting.graphql.snippets import SchemaSelection
from posting.help_data import HelpData
from posting.widgets.input import PostingInput
from posting.widgets.tree import PostingTree

NodeKind = Literal["group", "field", "type", "enum-value", "input-field"]

OPERATIONS: tuple[OperationType, ...] = ("query", "mutation", "subscription")

MAX_FIELD_MATCHES = 200
"""The most search results to show before asking for a narrower filter."""


@dataclass(frozen=True)
class NodeData:
    """What a node in the schema tree represents."""

    kind: NodeKind
    type_name: str = ""
    """The type which owns a field, or the type itself for a type node."""

    field_name: str = ""
    named_type: str = ""
    """The type a node expands into."""

    operation: OperationType | None = None

    @property
    def selection(self) -> SchemaSelection | None:
        """The selection to return when this node is chosen, if any."""
        if self.kind != "field":
            return None
        return SchemaSelection(
            type_name=self.type_name,
            field_name=self.field_name,
            operation=self.operation,
        )


def _arguments_signature(arguments: tuple[Argument, ...]) -> str:
    if not arguments:
        return ""
    return "(" + ", ".join(f"{a.name}: {a.type}" for a in arguments) + ")"


def _field_label(field: Field) -> Text:
    label = Text(field.name, style="bold" if not field.deprecated else "dim")
    label.append(_arguments_signature(field.arguments), style="dim")
    label.append(f": {field.type}", style="dim")
    if field.deprecated:
        label.append("  deprecated", style="dim italic")
    return label


def _match_label(match: FieldMatch) -> Text:
    """The label of a search result, which names the type that owns the field."""
    label = Text(f"{match.type_name}.", style="dim")
    label.append_text(_field_label(match.field))
    return label


def _type_label(type_: Type) -> Text:
    label = Text(type_.name, style="bold")
    label.append(f"  {type_.kind.lower().replace('_', ' ')}", style="dim")
    return label


class SchemaTree(PostingTree[NodeData]):
    """The tree of operations and types in a schema."""

    BINDING_GROUP_TITLE = "GraphQL Schema Tree"

    BINDINGS = [
        # `PostingTree` selects on `h`, which here would insert into the query -
        # moving to the parent node is far less surprising.
        Binding("h", "cursor_parent", "Go to parent", show=False),
    ]

    help = HelpData(
        title="GraphQL Schema Tree",
        description="""\
The types and operations supported by the endpoint.

Expand a field to walk into the type it returns.

Press `enter` on a field to insert it into the query editor: root fields of
`query`, `mutation` and `subscription` become a complete operation, and any
other field becomes a selection you can drop inside an existing one.
""",
    )


class GraphQLSchemaBrowser(ModalScreen[SchemaSelection | None]):
    """Displays a whole GraphQL schema, and inserts what the user picks."""

    BINDING_GROUP_TITLE = "GraphQL Schema Browser"

    DEFAULT_CSS = """
    GraphQLSchemaBrowser {
        align: center middle;

        /* Not `.modal-body`, since the app stylesheet sizes that for small
           dialogs and takes priority over a widget's own CSS. */
        & #schema-browser-body {
            width: 90%;
            height: 85%;
            padding: 0 1;
            background: $background;
            border: wide $background-lighten-2;
            border-title-color: $text;
            border-title-background: $background;
            border-title-style: bold;
        }

        & #schema-filter {
            dock: top;
            border: none;
            padding: 0 1;
            height: 1;
            background: transparent;
        }

        & #schema-browser-hints {
            dock: bottom;
            width: 1fr;
            padding: 0 1;
            color: $text-muted;
            background: $surface;
        }

        & #schema-tree {
            width: 1fr;
            padding: 0 1;
            background: transparent;
        }

        & #schema-detail {
            width: 1fr;
            padding: 0 1;
            border-left: solid $border;
            background: transparent;
        }
    }
    """

    BINDINGS = [
        Binding("escape", "close", "Close", show=False),
        Binding("slash,ctrl+f", "focus_filter", "Filter", show=False),
    ]

    def __init__(self, schema: Schema, endpoint: str = "") -> None:
        """
        Args:
            schema: The schema to display.
            endpoint: The URL the schema was fetched from, shown in the title.
        """
        super().__init__()
        self.schema = schema
        self.endpoint = endpoint

    def compose(self) -> ComposeResult:
        with Vertical(id="schema-browser-body") as body:
            body.border_title = self.endpoint or "GraphQL schema"
            yield PostingInput(
                placeholder="Filter fields and types",
                id="schema-filter",
            )
            yield Label(
                "enter Insert   space Expand   / Filter   esc Close",
                id="schema-browser-hints",
            )
            with Horizontal():
                tree: SchemaTree = SchemaTree(label="schema", id="schema-tree")
                tree.show_root = False
                tree.guide_depth = 2
                tree.auto_expand = False
                yield tree
                with VerticalScroll(id="schema-detail"):
                    yield Static(id="schema-detail-content")

    def on_mount(self) -> None:
        self.populate()
        self.tree.focus()

    # -- Building the tree ------------------------------------------------

    def populate(self, filter_text: str = "") -> None:
        """Rebuild the tree, optionally showing only entries matching a filter."""
        needle = filter_text.strip().lower()
        tree = self.tree
        tree.clear()
        root = tree.root

        for operation in OPERATIONS:
            type_ = self.schema.root_type(operation)
            if type_ is None:
                continue
            fields = [
                field
                for field in type_.fields.values()
                if not needle or needle in field.name.lower()
            ]
            if not fields:
                continue
            group = root.add(
                Text.assemble(
                    (operation, "bold"),
                    (f"  {type_.name}", "dim"),
                ),
                data=NodeData(kind="group", type_name=type_.name),
                expand=bool(needle) or operation == "query",
            )
            for field in fields:
                self._add_field(group, type_.name, field, operation)

        if needle:
            self._add_search_results(root, needle)

        type_names = [
            name
            for name in sorted(self.schema.types)
            if not name.startswith("__") and (not needle or needle in name.lower())
        ]
        if type_names:
            types_group = root.add(
                Text.assemble(
                    ("types", "bold"),
                    (f"  {len(type_names)}", "dim"),
                ),
                data=NodeData(kind="group"),
                expand=bool(needle),
            )
            for name in type_names:
                type_ = self.schema.types[name]
                types_group.add(
                    _type_label(type_),
                    data=NodeData(kind="type", type_name=name, named_type=name),
                    allow_expand=self._has_children(type_),
                )

        if tree.root.children:
            tree.cursor_line = 0
            self._show_detail(tree.root.children[0])
        else:
            self.detail.update(Text("Nothing matches that filter.", style="dim"))

    def _add_search_results(self, root: TreeNode[NodeData], needle: str) -> None:
        """Add the fields of any type which match the filter.

        The root fields are left out - they're already listed under their
        operation, just above.
        """
        root_types = {
            type_.name
            for operation in OPERATIONS
            if (type_ := self.schema.root_type(operation)) is not None
        }
        matches, total = self.schema.find_fields(
            needle, limit=MAX_FIELD_MATCHES, exclude_types=root_types
        )
        if not matches:
            return

        group = root.add(
            Text.assemble(("fields", "bold"), (f"  {total}", "dim")),
            data=NodeData(kind="group"),
            expand=True,
        )
        for match in matches:
            self._add_field(
                group,
                match.type_name,
                match.field,
                label=_match_label(match),
            )
        if total > len(matches):
            group.add_leaf(
                Text(
                    f"… {total - len(matches)} more - narrow the filter",
                    style="dim italic",
                ),
                data=NodeData(kind="group"),
            )

    def _add_field(
        self,
        parent: TreeNode[NodeData],
        owner_type: str,
        field: Field,
        operation: OperationType | None = None,
        label: Text | None = None,
    ) -> None:
        named_type = self.schema.type_named(field.named_type)
        parent.add(
            label if label is not None else _field_label(field),
            data=NodeData(
                kind="field",
                type_name=owner_type,
                field_name=field.name,
                named_type=field.named_type,
                operation=operation,
            ),
            allow_expand=named_type is not None and self._has_children(named_type),
        )

    def _has_children(self, type_: Type) -> bool:
        return bool(
            type_.fields
            or type_.input_fields
            or type_.enum_values
            or type_.possible_types
        )

    @on(Tree.NodeExpanded)
    def on_node_expanded(self, event: Tree.NodeExpanded[NodeData]) -> None:
        """Fill in a node's children the first time it's expanded."""
        node = event.node
        data = node.data
        if node.children or data is None or data.kind not in {"field", "type"}:
            return

        type_ = self.schema.type_named(data.named_type or data.type_name)
        if type_ is None:
            return

        for field in type_.fields.values():
            self._add_field(node, type_.name, field)
        for input_field in type_.input_fields.values():
            node.add_leaf(
                Text.assemble(
                    (input_field.name, "bold"), (f": {input_field.type}", "dim")
                ),
                data=NodeData(kind="input-field", type_name=type_.name),
            )
        for enum_value in type_.enum_values:
            node.add_leaf(
                Text(enum_value.name, style="dim" if enum_value.deprecated else ""),
                data=NodeData(kind="enum-value", type_name=type_.name),
            )
        for possible_type in type_.possible_types:
            possible = self.schema.type_named(possible_type)
            if possible is None:
                continue
            node.add(
                _type_label(possible),
                data=NodeData(
                    kind="type", type_name=possible_type, named_type=possible_type
                ),
                allow_expand=self._has_children(possible),
            )

    # -- Interaction -------------------------------------------------------

    @on(Tree.NodeHighlighted)
    def on_node_highlighted(self, event: Tree.NodeHighlighted[NodeData]) -> None:
        self._show_detail(event.node)

    @on(Tree.NodeSelected)
    def on_node_selected(self, event: Tree.NodeSelected[NodeData]) -> None:
        """Insert the selected field, or expand anything else."""
        node = event.node
        data = node.data
        selection = data.selection if data else None
        if selection is None:
            if node.allow_expand:
                node.toggle()
            return
        self.dismiss(selection)

    @on(Input.Changed, "#schema-filter")
    def on_filter_changed(self, event: Input.Changed) -> None:
        self.populate(event.value)

    @on(Input.Submitted, "#schema-filter")
    def on_filter_submitted(self) -> None:
        self.tree.focus()

    def action_focus_filter(self) -> None:
        self.query_one("#schema-filter", Input).focus()

    def action_close(self) -> None:
        self.dismiss(None)

    # -- The detail pane ---------------------------------------------------

    def _show_detail(self, node: TreeNode[NodeData]) -> None:
        data = node.data
        if data is None:
            return
        if data.kind == "field":
            field = self.schema.field(data.type_name, data.field_name)
            self.detail.update(self._field_detail(data, field) if field else Text(""))
        else:
            # For everything else - a type, an enum value, an input field, or
            # one of the groups - describe the type it belongs to.
            type_ = self.schema.type_named(data.type_name)
            self.detail.update(
                self._type_detail(type_) if type_ else self._schema_detail()
            )

    def _schema_detail(self) -> Text:
        """A summary of the schema, shown when nothing specific is selected."""
        kinds: dict[str, int] = {}
        for name, type_ in self.schema.types.items():
            if not name.startswith("__"):
                kinds[type_.kind] = kinds.get(type_.kind, 0) + 1

        detail = Text()
        detail.append(f"{self.endpoint or 'GraphQL schema'}\n", style="bold")
        detail.append(f"{self.schema.type_count} types\n", style="dim")
        for kind, count in sorted(kinds.items()):
            detail.append(f"  {count} {kind.lower().replace('_', ' ')}\n")
        detail.append(
            "\nExpand an operation to see its root fields, or a field to walk "
            "into the type it returns.",
            style="dim italic",
        )
        return detail

    def _field_detail(self, data: NodeData, field: Field) -> Text:
        detail = Text()
        detail.append(f"{data.type_name}.{field.name}\n", style="bold")
        detail.append(f"{field.type}\n", style="dim")
        if field.deprecated:
            detail.append("\ndeprecated\n", style="italic")
        if field.description:
            detail.append(f"\n{field.description}\n")
        if field.arguments:
            detail.append("\narguments\n", style="bold")
            for argument in field.arguments:
                detail.append(f"  {argument.name}", style="bold")
                detail.append(f": {argument.type}\n", style="dim")
                if argument.default_value:
                    detail.append(
                        f"    default: {argument.default_value}\n", style="dim"
                    )
                if argument.description:
                    detail.append(f"    {argument.description}\n")
        if data.operation:
            detail.append(
                f"\nPress enter to insert a {data.operation} which selects this field.",
                style="dim italic",
            )
        else:
            detail.append(
                "\nPress enter to insert this field as a selection.",
                style="dim italic",
            )
        return detail

    def _type_detail(self, type_: Type) -> Text:
        detail = Text()
        detail.append(f"{type_.name}\n", style="bold")
        detail.append(f"{type_.kind.lower().replace('_', ' ')}\n", style="dim")
        if type_.description:
            detail.append(f"\n{type_.description}\n")

        if type_.fields:
            detail.append("\nfields\n", style="bold")
            for field in type_.fields.values():
                detail.append(f"  {field.name}", style="bold")
                detail.append(
                    f"{_arguments_signature(field.arguments)}: {field.type}\n",
                    style="dim",
                )
        if type_.input_fields:
            detail.append("\ninput fields\n", style="bold")
            for input_field in type_.input_fields.values():
                detail.append(f"  {input_field.name}", style="bold")
                detail.append(f": {input_field.type}\n", style="dim")
        if type_.enum_values:
            detail.append("\nvalues\n", style="bold")
            for enum_value in type_.enum_values:
                detail.append(f"  {enum_value.name}\n")
        if type_.possible_types:
            detail.append("\npossible types\n", style="bold")
            for possible_type in type_.possible_types:
                detail.append(f"  {possible_type}\n")
        return detail

    @property
    def tree(self) -> SchemaTree:
        return self.query_one("#schema-tree", SchemaTree)

    @property
    def detail(self) -> Static:
        return self.query_one("#schema-detail-content", Static)
