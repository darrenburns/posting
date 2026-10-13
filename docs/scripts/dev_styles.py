"""Inline local stylesheets during `mkdocs serve` to avoid latency round trips.

Production builds retain the original stylesheet links. CSS stays in its source
files and MkDocs' normal rebuild/live-reload loop picks up edits.
"""

import html
import posixpath
import re
from pathlib import Path
from urllib.parse import urlsplit


serving = False
STYLESHEET = re.compile(r'<link rel="stylesheet" href="([^"]+)"\s*/?>')
CSS_URL = re.compile(r'url\(\s*([\'"]?)(.*?)\1\s*\)', re.DOTALL)


def on_startup(command, **kwargs):
    global serving
    serving = command == "serve"


def on_post_page(output, page, config):
    if not serving:
        return output

    def inline(match):
        href = html.unescape(match[1])
        if urlsplit(href).scheme or href.startswith("/"):
            return match[0]
        path = posixpath.normpath(posixpath.join(posixpath.dirname(page.file.dest_uri), href))
        css = (Path(config.site_dir) / path).read_text(encoding="utf-8")

        def rebase(url_match):
            url = url_match[2]
            if urlsplit(url).scheme or url.startswith(("/", "#")):
                return url_match[0]
            url = posixpath.normpath(posixpath.join(posixpath.dirname(href), url))
            return f'url("{url}")'

        css = CSS_URL.sub(rebase, css)
        return f'<style data-dev-stylesheet="{html.escape(href, quote=True)}">{css}</style>'

    return STYLESHEET.sub(inline, output)
