"""Build a review gallery for the candidate dark themes.

The captures come from `TestGenerateThemeCandidates` in internal/ui. Each
scene's markup is written once; every colour in it is a CSS variable slot
(see home_screens.py), and each card sets those slots for its own theme.

Usage: python docs/scripts/theme_gallery.py CAPTURE_DIR OUT_HTML
"""

from __future__ import annotations

import html
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from home_screens import Slots, render_grid  # noqa: E402

DOCS = Path(__file__).resolve().parent.parent
SCENES = [
    ("response", "Response"),
    ("palette", "Command palette"),
    ("history", "History"),
]
SWATCHES = ["Background", "Surface", "Primary", "Secondary", "Accent", "Success", "Warning", "Error"]


def main(capture_dir: Path, out: Path) -> None:
    scenes = json.loads((capture_dir / "scenes.json").read_text())
    themes = json.loads((capture_dir / "themes.json").read_text())
    names = [t["name"] for t in themes]
    colours = [t["colors"] for t in themes]
    slots = Slots()

    templates = []
    for key, _ in SCENES:
        by_theme = scenes[key]
        first = by_theme[names[0]]
        chars = [c.get("c", " ") for c in first["cells"]]
        for name in names:
            if [c.get("c", " ") for c in by_theme[name]["cells"]] != chars:
                raise SystemExit(f"{key}: {name} renders different characters from {names[0]}")
        grid = render_grid(first["w"], first["h"], [by_theme[n]["cells"] for n in names], colours, slots)
        templates.append(
            f'<template id="scene-{key}"><div class="tui-grid" style="--cols:{first["w"]};--rows:{first["h"]}">'
            f"{grid}</div></template>"
        )

    theme_css = []
    for index, (name, c) in enumerate(zip(names, colours)):
        decls = [f"--s{i}:{combo[index]}" for combo, i in slots.ids.items()]
        decls += [
            f"--t-bg:{c['Background'].lower()}",
            f"--t-primary:{c['Primary'].lower()}",
            f"--t-on-primary:{c['TextOnPrimary'].lower()}",
        ]
        theme_css.append(f'.th-{name}{{{";".join(decls)}}}')
    count = len(slots.ids)
    slot_css = [f".f{i}{{color:var(--s{i})}}" for i in range(count)]
    slot_css += [f".b{i}{{background-color:var(--s{i})}}" for i in range(count)]

    cards = []
    for name, theme, c in zip(names, themes, colours):
        swatches = "".join(
            f'<i style="background:{c[k].lower()}" title="{k} {c[k].lower()}"></i>' for k in SWATCHES
        )
        if theme.get("reference"):
            cards.append(
                f'<article class="card ref th-{name}" data-name="{name}">'
                f'<button type="button" class="shot" aria-label="Enlarge {name}"><div class="tui"></div></button>'
                f'<div class="meta"><div class="meta-text"><h2>{name}</h2>'
                f'<p>{html.escape(theme["blurb"])}</p><div class="swatches">{swatches}</div></div></div></article>'
            )
            continue
        cards.append(
            f'<article class="card th-{name}" data-name="{name}">'
            f'<button type="button" class="shot" aria-label="Enlarge {name}"><div class="tui"></div></button>'
            f'<div class="meta"><div class="meta-text"><h2>{name}</h2>'
            f'<p>{html.escape(theme["blurb"])}</p><div class="swatches">{swatches}</div></div>'
            f'<button type="button" class="keep" aria-pressed="false" aria-label="Keep {name}">'
            f'<span class="box" aria-hidden="true"></span><span class="lbl"></span></button></div></article>'
        )

    scene_buttons = "".join(
        f'<button type="button" role="radio" data-scene="{key}" aria-checked="{"true" if i == 0 else "false"}">'
        f'<kbd>{i + 1}</kbd>{label}</button>'
        for i, (key, label) in enumerate(SCENES)
    )

    screens_css = (DOCS / "stylesheets" / "screens.css").read_text().split("/* A screen embedded")[0]
    page = (
        (Path(__file__).resolve().parent / "theme_gallery.html")
        .read_text()
        .replace("/*SCREENS_CSS*/", screens_css)
        .replace("/*SLOT_CSS*/", "\n".join(slot_css))
        .replace("/*THEME_CSS*/", "\n".join(theme_css))
        .replace("<!--SCENE_BUTTONS-->", scene_buttons)
        .replace("<!--CARDS-->", "\n".join(cards))
        .replace("<!--TEMPLATES-->", "\n".join(templates))
        .replace("__COUNT__", str(sum(not t.get("reference") for t in themes)))
    )
    out.write_text(page)
    print(f"{len(names)} themes, {len(SCENES)} scenes, {count} colour slots, {len(page) / 1e6:.2f} MB")


if __name__ == "__main__":
    main(Path(sys.argv[1]), Path(sys.argv[2]))
