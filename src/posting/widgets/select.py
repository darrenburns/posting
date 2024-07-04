from typing import TypeVar
from textual.widgets import Select

T = TypeVar("T")


class PostingSelect(Select[T], inherit_bindings=False):
    BINDINGS = [("enter,space", "show_overlay")]
