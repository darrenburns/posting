from dataclasses import dataclass
from string import Template

from textual import events, on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.css.query import NoMatches
from textual.message import Message
from textual.widgets import Input, Label, TextArea

from posting.collection import GraphQLBody
from posting.graphql.cache import CachedSchema, get_schema
from posting.graphql.completion import analyze
from posting.graphql.highlighting import highlight_lines
from posting.graphql.schema import Schema
from posting.graphql.snippets import SchemaSelection, Snippet, snippet_for
from posting.help_data import HelpData
from posting.urls import ensure_protocol
from posting.variables import get_variables
from posting.widgets.request.graphql_autocomplete import GraphQLAutoComplete
from posting.widgets.text_area import PostingTextArea
from posting.widgets.variable_input import VariableInput


TRUNCATED_MESSAGE = (
    "The type was too large to select every field, so some were left out."
)
"""Shown when a generated selection hit the field limit."""


def resolve_endpoint_url(url: str) -> str:
    """Resolve the URL in the URL bar into the endpoint a schema is cached against.

    Variables are substituted where they're defined, and left alone where
    they're not, so that a partially resolved URL still returns something
    stable to look the schema up with.
    """
    url = url.strip()
    if not url:
        return ""
    try:
        url = Template(url).safe_substitute(get_variables())
    except ValueError:
        # An invalid template, e.g. a lone `$` - use the URL as written.
        pass
    return ensure_protocol(url)


class GraphQLQueryTextArea(PostingTextArea):
    """For editing GraphQL queries and mutations."""

    BINDING_GROUP_TITLE = "GraphQL Query Text Area"

    autocomplete: GraphQLAutoComplete | None = None
    """The completion dropdown, which is mounted alongside this text area."""

    BINDINGS = [
        Binding(
            "ctrl+space",
            "show_completions",
            "Suggest",
            tooltip="Suggest fields from the schema at the cursor.",
            id="graphql-suggest",
        ),
    ]

    help = HelpData(
        title="GraphQL Query Text Area",
        description="""\
A text area for entering a GraphQL query or mutation.

Write the query exactly as you would in any GraphQL editor - Posting will
escape it and wrap it in the JSON payload that GraphQL servers expect when
the request is sent.

If a schema has been fetched for this endpoint (press `f5` to fetch one),
fields, arguments, enum values and variables are suggested as you type.
Press `ctrl+space` to ask for suggestions at the cursor, `up` and `down` to
move through them, `enter` or `tab` to insert one, and `escape` to dismiss
them.

Press `ESC` to shift focus.
""",
    )

    def on_mount(self) -> None:
        self.tab_behavior = "indent"
        self.show_line_numbers = True
        self.autocomplete = GraphQLAutoComplete(
            target=self,
            schema_provider=self._get_schema,
        )
        self.screen.mount(self.autocomplete)

    def _build_highlight_map(self) -> None:
        """Syntax highlight the query using Posting's GraphQL lexer.

        `TextArea` highlights with tree-sitter, and Textual doesn't bundle a
        GraphQL grammar, so we fill in the highlight map ourselves. This runs
        wherever the tree-sitter path would - when the document is set, and
        after every edit.
        """
        self._line_cache.clear()
        self._highlights.clear()
        self._highlights.update(highlight_lines(self.document.lines))

    def _get_schema(self) -> Schema | None:
        """The schema to complete against, if one has been fetched."""
        try:
            editor = self.query_ancestor(GraphQLEditor)
        except NoMatches:
            return None
        cached = editor.cached_schema()
        return cached.schema if cached else None

    def on_key(self, event: events.Key) -> None:
        """Drive the completion dropdown.

        A text area acts on keys such as `enter` itself, so the dropdown can't
        intercept them - we do it here instead, and only while it's open.
        """
        autocomplete = self.autocomplete
        if autocomplete is None or not autocomplete.is_open:
            return

        if event.key in {"enter", "tab"}:
            autocomplete.accept()
        elif event.key == "down":
            autocomplete.highlight_next()
        elif event.key == "up":
            autocomplete.highlight_previous()
        elif event.key == "escape":
            autocomplete.action_hide()
        else:
            return

        # Stop the text area (and any bindings) acting on the key too.
        event.prevent_default()
        event.stop()

    def action_show_completions(self) -> None:
        autocomplete = self.autocomplete
        if autocomplete is None:
            return
        autocomplete.open()
        if not autocomplete.is_open:
            self.notify(
                "No suggestions available here. Press f5 to fetch the schema.",
                title="GraphQL",
                severity="information",
            )


class GraphQLVariablesTextArea(PostingTextArea):
    """For editing the variables sent alongside a GraphQL query."""

    BINDING_GROUP_TITLE = "GraphQL Variables Text Area"

    help = HelpData(
        title="GraphQL Variables Text Area",
        description="""\
A text area for entering the variables which accompany a GraphQL query,
written as a JSON object.

Press `ESC` to shift focus.
""",
    )

    def on_mount(self) -> None:
        self.tab_behavior = "indent"
        self.show_line_numbers = True


class GraphQLEditor(Vertical):
    """An editor for GraphQL request bodies.

    Holds the query (or mutation), the variables that accompany it, and
    optionally the name of the operation to execute.
    """

    BINDING_GROUP_TITLE = "GraphQL Editor"

    BINDINGS = [
        Binding(
            "f5",
            "screen.fetch_graphql_schema",
            "Fetch schema",
            tooltip="Fetch the GraphQL schema from this endpoint, enabling autocompletion.",
            id="fetch-graphql-schema",
        ),
        Binding(
            "f2",
            "screen.browse_graphql_schema",
            "Browse schema",
            tooltip="Browse the schema of this endpoint, and insert operations from it.",
            id="browse-graphql-schema",
        ),
    ]

    @dataclass
    class Changed(Message):
        """Posted when any part of the GraphQL body changes."""

        has_content: bool
        """True if any part of the GraphQL body has been filled in."""

        graphql_editor: "GraphQLEditor"

        @property
        def control(self) -> "GraphQLEditor":
            return self.graphql_editor

    DEFAULT_CSS = """
    GraphQLEditor {
        & #graphql-operation-name-container {
            dock: top;
            height: 1;

            & Label {
                padding: 0 1;
                color: $text-muted;
            }

            & #graphql-operation-name {
                height: 1;
                width: 1fr;
                border: none;
                padding: 0;
                background: transparent;
            }

            & #graphql-schema-status {
                dock: right;
                width: auto;
                padding: 0 1;
                color: $text-muted;
            }
        }

        & .graphql-section-label {
            padding: 0 1;
            width: 1fr;
            color: $text-muted;
            background: $surface-darken-1;
        }

        & GraphQLQueryTextArea {
            height: 2fr;
            background: transparent;
        }

        & GraphQLVariablesTextArea {
            height: 1fr;
            background: transparent;
        }
    }
    """

    def compose(self) -> ComposeResult:
        with Horizontal(id="graphql-operation-name-container"):
            yield Label("Operation")
            yield VariableInput(
                placeholder="Operation name (optional)",
                id="graphql-operation-name",
            )
            yield Label("no schema", id="graphql-schema-status")
        yield Label("Query", classes="graphql-section-label")
        yield GraphQLQueryTextArea(id="graphql-query", language=None)
        yield Label("Variables (JSON)", classes="graphql-section-label")
        yield GraphQLVariablesTextArea(id="graphql-variables", language="json")

    @on(TextArea.Changed)
    def on_text_area_changed(self, event: TextArea.Changed) -> None:
        """Let the app know whether this editor holds a body or not."""
        event.stop()
        self.post_message(self.Changed(self.has_content, self))

    @on(Input.Changed, selector="#graphql-operation-name")
    def on_operation_name_changed(self, event: Input.Changed) -> None:
        event.stop()
        self.post_message(self.Changed(self.has_content, self))

    @property
    def has_content(self) -> bool:
        """True if any part of the GraphQL body has been filled in."""
        return bool(
            self.query_text_area.text
            or self.variables_text_area.text
            or self.operation_name_input.value
        )

    def on_mount(self) -> None:
        self.refresh_schema_status()

    def on_show(self) -> None:
        # The endpoint may have changed while this editor was hidden.
        self.refresh_schema_status()

    def endpoint_url(self) -> str:
        """The endpoint this query will be sent to, as it's currently written."""
        try:
            url_input = self.screen.query_one("#url-input", Input)
        except NoMatches:
            return ""
        return resolve_endpoint_url(url_input.value)

    def cached_schema(self) -> CachedSchema | None:
        """The schema which has been fetched for this endpoint, if any."""
        return get_schema(self.endpoint_url())

    def refresh_schema_status(self) -> None:
        """Update the label which says whether we have a schema to complete against."""
        try:
            status = self.query_one("#graphql-schema-status", Label)
        except NoMatches:
            return
        cached = self.cached_schema()
        status.update(f"{cached.schema.type_count} types" if cached else "no schema")

    def insert_selection(self, schema: Schema, selection: SchemaSelection) -> None:
        """Insert a field picked in the schema browser into the query editor.

        A root field of an operation becomes a whole operation, unless the
        cursor is already inside a selection set - in which case, like any
        other field, it's inserted as a selection at the cursor.
        """
        text_area = self.query_text_area
        text = text_area.text
        cursor = text_area.document.get_index_from_location(text_area.cursor_location)

        snippet = snippet_for(schema, selection, text, cursor)
        if snippet is None:
            self.notify(
                severity="error",
                title="Couldn't insert field",
                message=f"{selection.field_name!r} is no longer in the schema.",
            )
        elif snippet.operation_name:
            self._insert_operation(snippet)
        else:
            declared = analyze(text, cursor).variables
            self._insert_selection_at_cursor(snippet, declared)

    def _insert_operation(self, snippet: Snippet) -> None:
        """Insert a complete operation, appending it if there's one already."""
        text_area = self.query_text_area
        existing = text_area.text.strip()
        if existing:
            # A document with more than one operation needs to say which of
            # them to run, so name the new one as the operation to send.
            text_area.text = f"{existing}\n\n{snippet.text}\n"
            self.operation_name_input.value = snippet.operation_name
            message = (
                f"Appended {snippet.operation_name}, and made it the operation to send."
            )
        else:
            text_area.text = f"{snippet.text}\n"
            message = f"Inserted {snippet.operation_name}."
        text_area.move_cursor(text_area.document.end)

        if snippet.variables and not self.variables_text_area.text.strip():
            self.variables_text_area.text = snippet.variables_json()
            message = f"{message} Fill in the variables below."
        if snippet.truncated:
            message = f"{message} {TRUNCATED_MESSAGE}"

        self.notify(title="GraphQL", message=message)

    def _insert_selection_at_cursor(
        self, snippet: Snippet, declared_variables: tuple[str, ...]
    ) -> None:
        """Insert a field selection at the cursor, indented to match."""
        text_area = self.query_text_area
        _row, column = text_area.cursor_location
        indent = "\n" + " " * column
        text_area.insert(indent.join(snippet.text.splitlines()))

        if snippet.truncated:
            self.notify(title="GraphQL", message=TRUNCATED_MESSAGE)

        undeclared = [
            name for name in snippet.variables if name not in declared_variables
        ]
        if undeclared:
            variables = ", ".join(
                f"${name}: {snippet.variables[name]}" for name in undeclared
            )
            self.notify(
                severity="warning",
                title="GraphQL",
                message=f"Add {variables} to the operation's variable definitions.",
            )

    @property
    def query_text_area(self) -> GraphQLQueryTextArea:
        return self.query_one("#graphql-query", GraphQLQueryTextArea)

    @property
    def variables_text_area(self) -> GraphQLVariablesTextArea:
        return self.query_one("#graphql-variables", GraphQLVariablesTextArea)

    @property
    def operation_name_input(self) -> VariableInput:
        return self.query_one("#graphql-operation-name", VariableInput)

    def to_model(self) -> GraphQLBody:
        return GraphQLBody(
            query=self.query_text_area.text,
            variables=self.variables_text_area.text,
            operation_name=self.operation_name_input.value,
        )

    def load_body(self, graphql: GraphQLBody | None) -> None:
        """Load a GraphQL body into the editor, clearing it if there is none."""
        graphql = graphql or GraphQLBody()
        self.query_text_area.text = graphql.query
        self.variables_text_area.text = graphql.variables
        self.operation_name_input.value = graphql.operation_name
        self.refresh_schema_status()
