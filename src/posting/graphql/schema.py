"""GraphQL schema introspection.

Holds the introspection query Posting sends to GraphQL endpoints, and a
lightweight model of the resulting schema which is used to power
autocompletion in the GraphQL query editor.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Collection, Iterable, Literal

OperationType = Literal["query", "mutation", "subscription"]

INTROSPECTION_QUERY = """\
query IntrospectionQuery {
  __schema {
    queryType { name }
    mutationType { name }
    subscriptionType { name }
    types {
      kind
      name
      description
      fields(includeDeprecated: true) {
        name
        description
        isDeprecated
        args { ...InputValue }
        type { ...TypeRef }
      }
      inputFields { ...InputValue }
      enumValues(includeDeprecated: true) {
        name
        description
        isDeprecated
      }
      possibleTypes { ...TypeRef }
    }
  }
}

fragment InputValue on __InputValue {
  name
  description
  defaultValue
  type { ...TypeRef }
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
              ofType {
                kind
                name
                ofType {
                  kind
                  name
                  ofType {
                    kind
                    name
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}
"""
"""The introspection query which Posting sends in order to fetch a schema.

A type reference is a chain of list and non-null wrappers around a named
type, which introspection returns as nested `ofType` objects. A query can't
recurse, so `TypeRef` spells the nesting out - nine levels, which is deeper
than any real schema goes (`[[Post!]!]!` needs four). A type nested deeper
than that comes back truncated, and Posting treats it as a type it doesn't
know: no completions are offered for it, and the schema browser can't walk
into it.
"""


class SchemaError(Exception):
    """Raised when an introspection response cannot be turned into a schema."""


@dataclass(frozen=True)
class Argument:
    """An argument accepted by a field, or a field of an input object."""

    name: str
    type: str
    """The type of the argument, rendered as it appears in a schema, e.g. `[ID!]!`."""

    named_type: str
    """The name of the type, with any list and non-null wrappers removed."""

    description: str = ""
    default_value: str | None = None


@dataclass(frozen=True)
class Field:
    """A field which may be selected on an object or interface type."""

    name: str
    type: str
    """The type of the field, rendered as it appears in a schema, e.g. `[Post!]!`."""

    named_type: str
    """The name of the type, with any list and non-null wrappers removed."""

    description: str = ""
    deprecated: bool = False
    arguments: tuple[Argument, ...] = ()

    def argument(self, name: str) -> Argument | None:
        for argument in self.arguments:
            if argument.name == name:
                return argument
        return None


@dataclass(frozen=True)
class EnumValue:
    name: str
    description: str = ""
    deprecated: bool = False


@dataclass(frozen=True)
class Type:
    """A type in a GraphQL schema."""

    name: str
    kind: str
    """One of OBJECT, INTERFACE, UNION, ENUM, INPUT_OBJECT or SCALAR."""

    description: str = ""
    fields: dict[str, Field] = field(default_factory=dict)
    input_fields: dict[str, Argument] = field(default_factory=dict)
    enum_values: tuple[EnumValue, ...] = ()
    possible_types: tuple[str, ...] = ()
    """The concrete types of a union or interface."""

    @property
    def is_selectable(self) -> bool:
        """Whether a selection set can be written for this type."""
        return self.kind in {"OBJECT", "INTERFACE", "UNION"}


@dataclass(frozen=True)
class FieldMatch:
    """A field found by searching the schema."""

    type_name: str
    """The type which owns the field."""

    field: Field

    @property
    def qualified_name(self) -> str:
        return f"{self.type_name}.{self.field.name}"


def _match_rank(search: str, type_name: str, field_name: str) -> int | None:
    """How well a field matches a search string. Lower is better, `None` misses."""
    name = field_name.lower()
    qualified = f"{type_name}.{field_name}".lower()
    if name.startswith(search):
        return 0
    if qualified.startswith(search):
        return 1
    if search in name:
        return 2
    if search in qualified:
        return 3
    return None


@dataclass
class Schema:
    """A GraphQL schema, as returned by an introspection query."""

    types: dict[str, Type] = field(default_factory=dict)
    query_type: str | None = None
    mutation_type: str | None = None
    subscription_type: str | None = None

    def type_named(self, name: str | None) -> Type | None:
        return self.types.get(name) if name else None

    def root_type(self, operation: OperationType) -> Type | None:
        """The type which an operation of the given kind selects fields from."""
        if operation == "mutation":
            return self.type_named(self.mutation_type)
        elif operation == "subscription":
            return self.type_named(self.subscription_type)
        return self.type_named(self.query_type)

    def field(self, type_name: str | None, field_name: str) -> Field | None:
        type_ = self.type_named(type_name)
        return type_.fields.get(field_name) if type_ else None

    def object_type_names(self) -> tuple[str, ...]:
        """The names of every type which can appear in a type condition."""
        return tuple(
            sorted(
                name
                for name, type_ in self.types.items()
                if type_.is_selectable and not name.startswith("__")
            )
        )

    def find_fields(
        self,
        search: str,
        limit: int | None = None,
        exclude_types: Collection[str] = (),
    ) -> tuple[tuple[FieldMatch, ...], int]:
        """Find the fields of any type whose name matches a search string.

        A field matches if the search string appears in its name, or in its
        name qualified by its type - so `user.na` finds `User.name`. Matches
        on the start of a name come first.

        Args:
            search: The string to search for, matched case-insensitively.
            limit: The most matches to return, or `None` for all of them.
            exclude_types: The names of types to leave out of the search.

        Returns:
            The matches, and the total number found (which is larger than the
            number returned when `limit` cuts the results short).
        """
        needle = search.strip().lower()
        if not needle:
            return (), 0

        ranked: list[tuple[int, str, str, FieldMatch]] = []
        for type_name, type_ in self.types.items():
            if type_name.startswith("__") or type_name in exclude_types:
                continue
            for field_ in type_.fields.values():
                rank = _match_rank(needle, type_name, field_.name)
                if rank is not None:
                    ranked.append(
                        (rank, type_name, field_.name, FieldMatch(type_name, field_))
                    )

        ranked.sort(key=lambda entry: entry[:3])
        matches = tuple(entry[3] for entry in ranked)
        return (matches if limit is None else matches[:limit]), len(matches)

    @property
    def type_count(self) -> int:
        """The number of types in the schema, excluding introspection types."""
        return sum(1 for name in self.types if not name.startswith("__"))


def render_type_ref(type_ref: dict[str, Any] | None) -> str:
    """Render an introspection type reference the way it's written in a schema.

    For example, a non-null list of non-null `Post` is rendered `[Post!]!`.
    """
    if not type_ref:
        return ""
    kind = type_ref.get("kind")
    if kind == "NON_NULL":
        return f"{render_type_ref(type_ref.get('ofType'))}!"
    elif kind == "LIST":
        return f"[{render_type_ref(type_ref.get('ofType'))}]"
    return type_ref.get("name") or ""


def named_type_of(type_ref: dict[str, Any] | None) -> str:
    """The name of a type reference, with list and non-null wrappers removed."""
    while type_ref:
        if name := type_ref.get("name"):
            return name
        type_ref = type_ref.get("ofType")
    return ""


def _parse_arguments(raw_arguments: Iterable[dict[str, Any]] | None) -> list[Argument]:
    return [
        Argument(
            name=raw["name"],
            type=render_type_ref(raw.get("type")),
            named_type=named_type_of(raw.get("type")),
            description=raw.get("description") or "",
            default_value=raw.get("defaultValue"),
        )
        for raw in raw_arguments or ()
        if raw.get("name")
    ]


def _parse_type(raw_type: dict[str, Any]) -> Type:
    fields: dict[str, Field] = {}
    for raw_field in raw_type.get("fields") or ():
        name = raw_field.get("name")
        if not name:
            continue
        fields[name] = Field(
            name=name,
            type=render_type_ref(raw_field.get("type")),
            named_type=named_type_of(raw_field.get("type")),
            description=raw_field.get("description") or "",
            deprecated=bool(raw_field.get("isDeprecated")),
            arguments=tuple(_parse_arguments(raw_field.get("args"))),
        )

    input_fields = {
        argument.name: argument
        for argument in _parse_arguments(raw_type.get("inputFields"))
    }

    enum_values = tuple(
        EnumValue(
            name=raw_value["name"],
            description=raw_value.get("description") or "",
            deprecated=bool(raw_value.get("isDeprecated")),
        )
        for raw_value in raw_type.get("enumValues") or ()
        if raw_value.get("name")
    )

    possible_types = tuple(
        name
        for raw_possible in raw_type.get("possibleTypes") or ()
        if (name := named_type_of(raw_possible))
    )

    return Type(
        name=raw_type["name"],
        kind=raw_type.get("kind") or "OBJECT",
        description=raw_type.get("description") or "",
        fields=fields,
        input_fields=input_fields,
        enum_values=enum_values,
        possible_types=possible_types,
    )


def parse_introspection(payload: dict[str, Any]) -> Schema:
    """Build a `Schema` from the JSON body of an introspection response.

    Args:
        payload: The decoded JSON body of the introspection response.

    Raises:
        SchemaError: If the response doesn't contain a schema.
    """
    if not isinstance(payload, dict):
        raise SchemaError("The endpoint didn't return a JSON object.")

    if errors := payload.get("errors"):
        messages = "; ".join(
            str(error.get("message", error)) for error in errors if error
        )
        raise SchemaError(messages or "The endpoint returned an error.")

    data = payload.get("data") or payload
    raw_schema = data.get("__schema") if isinstance(data, dict) else None
    if not isinstance(raw_schema, dict):
        raise SchemaError("The response didn't contain a GraphQL schema.")

    types = {
        raw_type["name"]: _parse_type(raw_type)
        for raw_type in raw_schema.get("types") or ()
        if raw_type.get("name")
    }
    if not types:
        raise SchemaError("The schema returned by the endpoint contains no types.")

    def root_name(key: str) -> str | None:
        root = raw_schema.get(key)
        return root.get("name") if isinstance(root, dict) else None

    return Schema(
        types=types,
        query_type=root_name("queryType"),
        mutation_type=root_name("mutationType"),
        subscription_type=root_name("subscriptionType"),
    )
