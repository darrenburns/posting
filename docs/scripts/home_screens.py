"""Turn Posting 3 screen captures into HTML for the docs homepage.

The captures come from `TestGenerateHomepageScenes` in internal/ui. Each
scene is rendered once per theme; cells keep the same characters across
themes and only their colours change. Every distinct combination of colours
across the themes becomes a numbered CSS variable ("slot"), so switching the
palette on the page re-colours the real UI without swapping any markup.

Usage: python docs/scripts/home_screens.py CAPTURE_DIR
"""

from __future__ import annotations

import html
import json
import sys
from pathlib import Path

DOCS = Path(__file__).resolve().parent.parent
SCREENS_DIR = DOCS / "overrides" / "partials" / "screens"
PALETTES_CSS_PATH = DOCS / "stylesheets" / "palettes.css"
CSS_PATH = DOCS / "stylesheets" / "home-palettes.css"

# The palettes offered across the docs, in picker order. The first is the
# default. All are built-in Posting 3 themes.
PALETTES = [
    "galaxy",
    "aurora",
    "lantern",
    "midnight-ember",
    "kintsugi",
    "neon-reef",
    "cyberdeck",
    "catppuccin-latte",
]

SCENE_LABELS = {
    "response": "Posting showing the List users request with a highlighted JSON response",
    "jump": "Jump mode, with a label on every pane, tab and request",
    "palette": "The command palette listing actions and their shortcuts",
    "themes": "The theme picker inside the command palette",
    "variables": "The variables screen for the local environment, with secrets masked",
    "curlhint": "A curl command pasted into the URL bar, ready to import",
    "curlexport": "A request exported as a multi-line curl command",
    "history": "Side-by-side layout with request history and a timing waterfall",
}

# Scenes whose characters differ between themes (the theme picker moves its
# checkmark to the active theme). These get one variant per palette.
PER_THEME_SCENES = {"themes"}

BOLD, ITALIC, REVERSE = 1, 4, 32

# Glyphs drawn with CSS so that lines join up whatever font is available.
LINES = {"─": "h", "━": "hh", "│": "v", "╭": "tl", "╮": "tr", "╰": "bl", "╯": "br"}
BLOCKS = {"▎": "q1", "▌": "q2", "▂": "l2", "▃": "l3", "▄": "l4", "▅": "l5", "▆": "l6", "▇": "l7"}
MERGEABLE = {"h", "hh"}


def in_web_font(ch: str) -> bool:
    """Whether the Google Fonts latin subset of the grid font covers ch."""
    cp = ord(ch)
    return cp < 0x100 or 0x2000 <= cp <= 0x206F


class Slots:
    def __init__(self) -> None:
        self.ids: dict[tuple[str, ...], int] = {}

    def get(self, colours: tuple[str, ...]) -> int:
        colours = tuple(c.lower() for c in colours)
        if colours not in self.ids:
            self.ids[colours] = len(self.ids)
        return self.ids[colours]


def cell_colours(cell: dict, theme: dict) -> tuple[str, str]:
    fg = cell.get("f") or theme["Text"]
    bg = cell.get("b") or theme["Background"]
    if cell.get("a", 0) & REVERSE:
        fg, bg = bg, fg
    return fg, bg


def render_grid(width: int, height: int, cells_by_theme: list[list[dict]], themes: list[dict], slots: Slots) -> str:
    """Render cells to rows of spans. cells_by_theme[i] holds theme i's cells."""
    chars = [c.get("c", " ") for c in cells_by_theme[0]]
    rows = []
    for y in range(height):
        runs: list[list] = []  # [kind, text, fg, bg, attrs, count]
        x = 0
        while x < width:
            i = y * width + x
            ch = chars[i] or " "
            per_theme = [cell_colours(cells[i], th) for cells, th in zip(cells_by_theme, themes)]
            fg = slots.get(tuple(c[0] for c in per_theme))
            bg = slots.get(tuple(c[1] for c in per_theme))
            attrs = cells_by_theme[0][i].get("a", 0) & (BOLD | ITALIC)
            wide = x + 1 < width and chars[i + 1] == ""
            if ch in LINES:
                kind = LINES[ch]
            elif ch in BLOCKS:
                kind = BLOCKS[ch]
            elif in_web_font(ch) and not wide:
                kind = "t"
            else:
                kind = "g2" if wide else "g"
            last = runs[-1] if runs else None
            if last and last[0] == kind and last[2:5] == [fg, bg, attrs] and (kind == "t" or kind in MERGEABLE):
                last[1] += ch
                last[5] += 1
            else:
                runs.append([kind, ch, fg, bg, attrs, 1])
            x += 2 if wide else 1
        spans = []
        for kind, text, fg, bg, attrs, count in runs:
            classes = [f"f{fg}", f"b{bg}"]
            if attrs & BOLD:
                classes.append("B")
            if attrs & ITALIC:
                classes.append("I")
            if kind == "t":
                body = html.escape(text).replace("{", "&#123;").replace("}", "&#125;")
                spans.append(f'<span class="{" ".join(classes)}">{body}</span>')
            elif kind in ("g", "g2"):
                classes.append(kind)
                spans.append(f'<span class="{" ".join(classes)}">{html.escape(text)}</span>')
            else:
                classes.append("d " + kind)
                spans.append(f'<span class="{" ".join(classes)}">{" " * count}</span>')
        rows.append('<div class="r">' + "".join(spans) + "</div>")
    return "".join(rows)


def main(capture_dir: Path) -> None:
    scenes = json.loads((capture_dir / "scenes.json").read_text())
    all_themes = json.loads((capture_dir / "themes.json").read_text())
    themes = [all_themes[name] for name in PALETTES]
    slots = Slots()

    SCREENS_DIR.mkdir(parents=True, exist_ok=True)
    for scene, by_theme in scenes.items():
        base = by_theme[PALETTES[0]]
        w, h = base["w"], base["h"]
        label = html.escape(SCENE_LABELS.get(scene, scene), quote=True)
        if scene in PER_THEME_SCENES:
            variants = []
            for name, theme in zip(PALETTES, themes):
                # Colours only matter while this variant's palette is active,
                # so the same colour stands in for every palette.
                grid = render_grid(w, h, [by_theme[name]["cells"]] * len(themes), [theme] * len(themes), slots)
                variants.append(f'<div class="tui-grid" data-for="{name}">{grid}</div>')
            body = "".join(variants)
        else:
            grid = render_grid(w, h, [by_theme[name]["cells"] for name in PALETTES], themes, slots)
            body = f'<div class="tui-grid">{grid}</div>'
        out = f'<div class="tui" data-scene="{scene}" role="img" aria-label="{label}" style="--cols:{w};--rows:{h}">{body}</div>\n'
        (SCREENS_DIR / f"{scene}.html").write_text(out)

    generated = "/* Generated by docs/scripts/home_screens.py. Do not edit by hand. */"
    # Every docs page loads the theme tokens. Only the homepage needs the
    # screen colours (--sN).
    tokens_css = [generated, "/* Palettes come from Posting 3's built-in themes. */"]
    css = [generated, "/* Screen colours for each palette in palettes.css. */"]
    for index, (name, theme) in enumerate(zip(PALETTES, themes)):
        selector = f':root[data-palette="{name}"]'
        if index == 0:
            selector = ":root, " + selector
        tokens = {
            "bg": theme["Background"],
            "surface": theme["Surface"],
            "panel": theme["SurfaceHover"],
            "surface-2": theme["Surface2"],
            "surface-3": theme["Surface3"],
            "border": theme["Border"],
            "text": theme["Text"],
            "muted": theme["TextMuted"],
            "dim": theme["TextDisabled"],
            "primary": theme["Primary"],
            "secondary": theme["Secondary"],
            "accent": theme["Accent"],
            "on-primary": theme["TextOnPrimary"],
            "on-accent": theme["TextOnAccent"],
            "primary-text": theme["PrimaryText"],
            "secondary-text": theme["SecondaryText"],
            "accent-text": theme["AccentText"],
            "success": theme["Success"],
            "success-text": theme["SuccessText"],
            "warning": theme["Warning"],
            "warning-text": theme["WarningText"],
            "error": theme["Error"],
            "error-text": theme["ErrorText"],
            "info": theme["Info"],
            "info-text": theme["InfoText"],
        }
        decls = [f"--{k}:{v.lower()}" for k, v in tokens.items()]
        decls.append("color-scheme:" + ("light" if theme["IsLight"] else "dark"))
        tokens_css.append(selector + "{" + ";".join(decls) + "}")
        slots_decls = [f"--s{i}:{colours[index]}" for colours, i in slots.ids.items()]
        css.append(selector + "{" + ";".join(slots_decls) + "}")
    PALETTES_CSS_PATH.write_text("\n".join(tokens_css) + "\n")
    count = len(slots.ids)
    variant_rules = [f':root[data-palette="{name}"] .tui-grid[data-for="{name}"]' for name in PALETTES]
    variant_rules.insert(0, f':root:not([data-palette]) .tui-grid[data-for="{PALETTES[0]}"]')
    css.append(".tui-grid[data-for]{display:none}")
    css.append(",".join(variant_rules) + "{display:block}")
    css += [f".f{i}{{color:var(--s{i})}}" for i in range(count)]
    css += [f".b{i}{{background-color:var(--s{i})}}" for i in range(count)]
    CSS_PATH.write_text("\n".join(css) + "\n")

    picker = ['<div class="palette-picker" role="radiogroup" aria-label="Theme">']
    for name, theme in zip(PALETTES, themes):
        swatches = "".join(
            f'<i style="background:{theme[key].lower()}"></i>' for key in ("Primary", "Secondary", "Accent")
        )
        picker.append(
            f'<button type="button" role="radio" aria-checked="false" data-palette="{name}"'
            f' style="--sw-bg:{theme["Background"].lower()};--sw-fg:{theme["Text"].lower()}">'
            f'<span class="sw">{swatches}</span><span class="nm">{name}</span></button>'
        )
    picker.append("</div>")
    (SCREENS_DIR / "palette-picker.html").write_text("".join(picker) + "\n")
    print(f"{len(scenes)} scenes, {len(PALETTES)} palettes, {count} colour slots")


if __name__ == "__main__":
    main(Path(sys.argv[1]))
