"""Shared helpers for styling and labelling HTTP methods consistently
across the UI.

Used by both the collection tree (`posting.widgets.collection.browser`)
and the request search palette (`posting.app`) so that a request's HTTP
method is visually distinguishable — by abbreviation and colour — no
matter where it's displayed.
"""

from __future__ import annotations

from posting.collection import VALID_HTTP_METHODS

# The theme variable used to colour a method when a theme doesn't define
# a bespoke `method-{method}` variable (e.g. `method-get`, `method-post`, ...).
_DEFAULT_STYLE_VARS: dict[str, str] = {
    "get": "text-primary",
    "post": "text-success",
    "put": "text-warning",
    "patch": "text-warning",
    "delete": "text-error",
    "options": "text-muted",
    "head": "text-muted",
}

MAX_METHOD_LENGTH = max(len(method) for method in VALID_HTTP_METHODS)
"""Length of the longest supported HTTP method name (`"OPTIONS"` -> 7).

Useful for aligning method labels in fixed-width UI contexts.
"""


def get_method_style(theme_variables: dict[str, str], method: str) -> str | None:
    """Return the style/colour to render `method` with, given the current theme.

    Looks up a theme-specific `method-{method}` variable first (themes can
    override individual method colours), falling back to a sensible default
    derived from the theme's semantic text colours.

    Args:
        theme_variables: `app.theme_variables` of the currently active theme.
        method: The HTTP method, e.g. `"GET"` or `"post"` (case-insensitive).

    Returns:
        A style/colour string suitable for Rich/Textual markup, or `None`
        if no style could be resolved.
    """
    method_lower = method.lower()
    default_var = _DEFAULT_STYLE_VARS.get(method_lower)
    default_style = theme_variables.get(default_var) if default_var else None
    return theme_variables.get(f"method-{method_lower}", default_style)


def get_method_abbreviation(method: str, length: int = 3) -> str:
    """Return a fixed-length, upper-case abbreviation of an HTTP method.

    Used for compact badges where space is limited, e.g. in the collection
    tree (`CollectionTree.render_label`).
    """
    return method.upper()[:length]


def get_method_label(method: str, *, pad: bool = False) -> str:
    """Return the full, upper-case name of an HTTP method.

    Args:
        method: The HTTP method, e.g. `"get"` or `"POST"`.
        pad: If `True`, right-pads the label with spaces to `MAX_METHOD_LENGTH`
            so labels line up in fixed-width UI contexts (e.g. a palette list).
    """
    label = method.upper()
    return label.ljust(MAX_METHOD_LENGTH) if pad else label
