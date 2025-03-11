from dataclasses import dataclass, field


@dataclass
class HelpData:
    """Data relating to the widget to be displayed in the HelpScreen"""

    title: str = field(default="")
    """Title of the widget"""
    description: str = field(default="")
    """Markdown description to be displayed in the HelpScreen"""
