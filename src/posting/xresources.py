import itertools
import subprocess
from typing import Any

from posting.themes import Theme
from textual.theme import Theme as TextualTheme

XRDB_MAPPING = {
    "color0": ["primary"],
    "color8": ["secondary"],
    "color1": ["error"],
    "color2": ["success"],
    "color3": ["warning"],
    "color4": ["accent"],
    "background": ["background"],
    "color7": ["surface", "panel"],
}


def load_xresources_themes() -> dict[str, TextualTheme]:
    """Runs xrdb -query and returns a dictionary of theme_name -> ColorSystem objects."""
    try:
        result = subprocess.run(
            ["xrdb", "-query"], capture_output=True, text=True, check=True
        )
    except subprocess.CalledProcessError as e:
        raise RuntimeError(f"Error running xrdb: {e}")
    except OSError as e:
        raise RuntimeError(f"Error running xrdb: {e}")

    supplied_colors: dict[str, Any] = {}
    for line in result.stdout.splitlines():
        if line.startswith("*"):
            name, value = line.removeprefix("*").split(":", 1)
            for kwarg in XRDB_MAPPING.get(name.strip(), []):
                supplied_colors[kwarg] = value.strip()

    missing_colors = (
        set(itertools.chain(*XRDB_MAPPING.values())) - supplied_colors.keys()
    )
    if missing_colors:
        missing_colors_string = ", ".join(missing_colors)
        raise RuntimeError(f"Missing colors from xrdb: {missing_colors_string}")

    return {
        "xresources-dark": Theme(
            name="xresources-dark",
            **supplied_colors,
            dark=True,
        ).to_textual_theme(),
        "xresources-light": Theme(
            name="xresources-light",
            **supplied_colors,
            dark=False,
        ).to_textual_theme(),
    }
