"""Generating GraphQL snippets from a schema.

Used by the schema browser: picking a field builds either a complete operation
(when starting from a root field) or the selection of that field, ready to be
inserted into the query editor.
"""

from __future__ import annotations

from dataclasses import dataclass, field as dataclass_field

from posting.graphql.completion import analyze
from posting.graphql.schema import Field, OperationType, Schema, Type

OPERATION_NAME_PREFIXES: dict[OperationType, str] = {
    "query": "Get",
    "mutation": "",
    "subscription": "On",
}

DEFAULT_DEPTH = 4
"""How many levels of nested objects to expand.

Four levels covers the shapes that APIs tend to have - a field, an object,
a list wrapper, and the items in it."""

MAX_FIELDS = 150
"""The most fields to put in one generated selection.

Without a cap, picking a field on a very wide type could paste thousands of
lines into the editor."""

TYPENAME = "__typename"
"""Selected when there's nothing else that can be selected on a type."""


@dataclass(frozen=True)
class SchemaSelection:
    """A field which a user picked out of a schema."""

    type_name: str
    """The type which owns the field."""

    field_name: str

    operation: OperationType | None = None
    """The kind of operation this field is a root field of, if it is one."""


@dataclass
class Snippet:
    """A chunk of GraphQL, ready to be inserted into the editor."""

    text: str
    """The generated GraphQL."""

    variables: dict[str, str] = dataclass_field(default_factory=dict)
    """The variables the snippet expects, mapping name to GraphQL type."""

    operation_name: str = ""
    """The name of the generated operation, if a whole operation was generated."""

    truncated: bool = False
    """True if the type was too large to select every field of."""

    def variables_json(self) -> str:
        """A JSON object with a null for each variable, to be filled in."""
        if not self.variables:
            return ""
        lines = [f'  "{name}": null,' for name in self.variables]
        lines[-1] = lines[-1].rstrip(",")
        return "\n".join(["{", *lines, "}"])


@dataclass
class _Budget:
    """Limits how many fields one generated selection can contain."""

    remaining: int = MAX_FIELDS
    truncated: bool = False

    def take(self) -> bool:
        """Claim room for one more field, if there is any."""
        if self.remaining <= 0:
            self.truncated = True
            return False
        self.remaining -= 1
        return True


class _Variables:
    """Allocates unique variable names for the arguments of a snippet."""

    def __init__(self) -> None:
        self.declared: dict[str, str] = {}

    def add(self, name: str, type_: str) -> str:
        """Declare a variable for an argument, returning the name to reference."""
        candidate = name
        suffix = 2
        while candidate in self.declared:
            candidate = f"{name}{suffix}"
            suffix += 1
        self.declared[candidate] = type_
        return candidate


def _is_required(argument_type: str) -> bool:
    return argument_type.endswith("!")


def _is_leaf(schema: Schema, field: Field) -> bool:
    """Whether a field can be selected without a selection set of its own.

    Scalars and enums can be. So can a field whose type isn't in the schema -
    we can't generate a selection set for a type we don't know about.
    """
    named_type = schema.type_named(field.named_type)
    return named_type is None or not named_type.is_selectable


def _arguments(field: Field, variables: _Variables) -> str:
    """Render the required arguments of a field, as references to variables."""
    required = [argument for argument in field.arguments if _is_required(argument.type)]
    if not required:
        return ""
    rendered = ", ".join(
        f"{argument.name}: ${variables.add(argument.name, argument.type)}"
        for argument in required
    )
    return f"({rendered})"


def _selection_lines(
    schema: Schema,
    type_: Type,
    variables: _Variables,
    budget: _Budget,
    depth: int,
    visited: frozenset[str],
) -> list[str]:
    """The lines of a selection set for `type_`, without the enclosing braces.

    Every field which can be selected is, in the order the schema declares
    them: fields which need no selection set of their own, and objects
    expanded into their own selection sets while `depth` allows it.
    """
    lines: list[str] = []
    for field in type_.fields.values():
        if field.deprecated:
            continue

        named_type = schema.type_named(field.named_type)
        if named_type is not None and named_type.is_selectable:
            # An object, interface or union: it can only be selected with a
            # selection set, so leave it out if we can't build one. Types
            # which contain themselves would otherwise recurse forever.
            if depth <= 0 or named_type.name in visited:
                continue

        if not budget.take():
            break

        lines.extend(
            _field_lines(
                schema, field, variables, budget, depth, visited | {type_.name}
            )
        )

    return lines or [TYPENAME]


def _field_lines(
    schema: Schema,
    field: Field,
    variables: _Variables,
    budget: _Budget,
    depth: int,
    visited: frozenset[str] = frozenset(),
) -> list[str]:
    """The lines which select `field`, including its own selection set."""
    header = f"{field.name}{_arguments(field, variables)}"
    named_type = schema.type_named(field.named_type)
    if named_type is None or not named_type.is_selectable:
        return [header]

    inner = _selection_lines(schema, named_type, variables, budget, depth - 1, visited)
    return [f"{header} {{", *[f"  {line}" for line in inner], "}"]


def build_selection(
    schema: Schema, type_name: str, field_name: str, depth: int = DEFAULT_DEPTH
) -> Snippet | None:
    """Build the selection of a single field, e.g. `user(id: $id) { id name }`.

    Args:
        schema: The schema to generate from.
        type_name: The type which owns the field.
        field_name: The field to select.
        depth: How many levels of nested objects to expand.

    Returns:
        The snippet, or `None` if the field isn't in the schema.
    """
    field = schema.field(type_name, field_name)
    if field is None:
        return None

    variables = _Variables()
    budget = _Budget()
    lines = _field_lines(schema, field, variables, budget, depth)
    return Snippet(
        text="\n".join(lines),
        variables=dict(variables.declared),
        truncated=budget.truncated,
    )


def operation_name_for(operation: OperationType, field_name: str) -> str:
    """A conventional name for an operation which selects `field_name`."""
    pascal_case = field_name[:1].upper() + field_name[1:]
    return f"{OPERATION_NAME_PREFIXES[operation]}{pascal_case}"


def build_operation(
    schema: Schema,
    operation: OperationType,
    field_name: str,
    depth: int = DEFAULT_DEPTH,
) -> Snippet | None:
    """Build a complete operation which selects a root field.

    For example, selecting the `user` field of the query root gives:

        query GetUser($id: ID!) {
          user(id: $id) {
            id
            name
          }
        }

    Args:
        schema: The schema to generate from.
        operation: The kind of operation to build.
        field_name: The root field to select.
        depth: How many levels of nested objects to expand.

    Returns:
        The snippet, or `None` if the field isn't a root field of the schema.
    """
    root = schema.root_type(operation)
    field = root.fields.get(field_name) if root else None
    if root is None or field is None:
        return None

    variables = _Variables()
    budget = _Budget()
    lines = _field_lines(schema, field, variables, budget, depth)
    declared = dict(variables.declared)
    signature = (
        "(" + ", ".join(f"${name}: {type_}" for name, type_ in declared.items()) + ")"
        if declared
        else ""
    )
    name = operation_name_for(operation, field_name)
    text = "\n".join(
        [
            f"{operation} {name}{signature} {{",
            *[f"  {line}" for line in lines],
            "}",
        ]
    )
    return Snippet(
        text=text,
        variables=declared,
        operation_name=name,
        truncated=budget.truncated,
    )


def snippet_for(
    schema: Schema,
    selection: SchemaSelection,
    document: str = "",
    cursor: int = 0,
    depth: int = DEFAULT_DEPTH,
) -> Snippet | None:
    """The snippet to insert for a picked field, given where the cursor is.

    A root field becomes a complete operation, unless the cursor is already
    inside a selection set - there, like any other field, it becomes a
    selection which can be dropped in as-is.

    Args:
        schema: The schema the field was picked from.
        selection: The field which was picked.
        document: The GraphQL document being edited.
        cursor: The offset of the cursor within that document.
        depth: How many levels of object fields to expand.

    Returns:
        The snippet, or `None` if the field is no longer in the schema.
    """
    inside_selection_set = analyze(document, cursor).kind in {"field", "type"}
    if selection.operation and not inside_selection_set:
        return build_operation(schema, selection.operation, selection.field_name, depth)
    return build_selection(schema, selection.type_name, selection.field_name, depth)
