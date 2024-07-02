from textual.widget import Widget


class CenterMiddle(Widget, inherit_bindings=False):
    """A container which aligns children on both axes."""

    DEFAULT_CSS = """
    CenterMiddle {
        align: center middle;
        width: 1fr;
        height: 1fr;
    }
    """
