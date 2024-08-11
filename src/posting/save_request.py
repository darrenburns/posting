from __future__ import annotations
import re


FILE_SUFFIX = ".posting.yaml"


def slugify(text: str) -> str:
    """Slugify a string."""
    text = text.lower()
    text = re.sub(r"[^a-z0-9]+", "-", text)
    return text.strip("-")


def generate_request_filename(request_title: str) -> str:
    """Generate a filename for a request, NOT including the file suffix."""
    return slugify(request_title)
