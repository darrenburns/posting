import re
from urllib.parse import urlparse, urlunparse


_PATH_PARAM_PATTERN = re.compile(r":([A-Za-z_][A-Za-z0-9_]*)")


def ensure_protocol(url: str) -> str:
    """Default the protocol to http:// if no protocol is present.

    Args:
        url: The URL to ensure has a protocol.

    Returns:
        The URL with a the http:// protocol if no protocol was present,
        otherwise the original URL.
    """
    if re.match(r"^[a-zA-Z][a-zA-Z0-9+.-]*://", url):
        return url
    return f"http://{url}"


def extract_path_param_names(url: str) -> list[str]:
    """Extract `:param` style placeholder names from the URL path, preserving order.

    Args:
        url: URL which may contain placeholders in the path like ":id".

    Returns:
        Ordered list of unique placeholder names as they appear in the path.
    """
    try:
        parsed = urlparse(url)
        path = parsed.path or ""
    except Exception:
        path = url

    seen: set[str] = set()
    ordered: list[str] = []
    for match in _PATH_PARAM_PATTERN.finditer(path):
        name = match.group(1)
        if name not in seen:
            seen.add(name)
            ordered.append(name)
    return ordered


def substitute_path_params(url: str, params: dict[str, str]) -> str:
    """Substitute `:param` placeholders in the URL path with provided values.

    Replacement only applies to the path portion, not query/fragment.

    Args:
        url: URL which may contain placeholders in the path like ":id".
        params: Mapping of placeholder name to replacement value.

    Returns:
        URL with placeholders replaced. Placeholders without a provided value are left intact.
    """
    try:
        parsed = urlparse(url)
        path = parsed.path or ""
    except Exception:
        # If parsing fails, do a simple best-effort replace on the whole string
        result = url
        for name, value in params.items():
            result = result.replace(f":{name}", value)
        return result

    new_path = path
    for name, value in params.items():
        # Simple literal replacement for ":name" within the path
        new_path = new_path.replace(f":{name}", value)

    return urlunparse(
        (
            parsed.scheme,
            parsed.netloc,
            new_path,
            parsed.params,
            parsed.query,
            parsed.fragment,
        )
    )
