from __future__ import annotations
import datetime
import re

from posting.collection import RequestModel

FILE_SUFFIX = ".posting.yaml"


def slugify(text: str) -> str:
    """Slugify a string."""
    text = text.lower()
    text = re.sub(r"[^a-z0-9]+", "-", text)
    return text.strip("-")


def generate_request_filename(request: RequestModel) -> str:
    """Generate a filename for a request."""
    method = request.method
    name = request.name
    timestamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    slug = f"-{slugify(name)}" if name else ""
    return f"{timestamp}-{method.lower()}{slug}"
