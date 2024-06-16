from __future__ import annotations
import datetime
import re


def slugify(text: str) -> str:
    """Slugify a string."""
    text = text.lower()
    text = re.sub(r"[^a-z0-9]+", "-", text)
    return text.strip("-")


def generate_request_filename(method: str, name: str | None = None) -> str:
    """Generate a filename for a request."""
    timestamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    slug = f"-{slugify(name)}" if name else ""
    return f"{timestamp}-{method.lower()}{slug}.posting.yaml"


# Example usage
if __name__ == "__main__":
    method = "POST"
    name = "Create User Profile"
    filename = generate_request_filename(method, name)
    print(filename)  # Output: "20231005-143045-post-create-user-profile.posting.yaml"
