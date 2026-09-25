import re
from collections import defaultdict, deque
from urllib.parse import parse_qsl, urlencode, urlparse, urlunparse


# Match a single-colon path param like ":id", but ignore escaped tokens like "::id".
_PATH_PARAM_PATTERN = re.compile(r"(?<!:):([A-Za-z_][A-Za-z0-9_]*)")


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
        # If parsing fails, do a best-effort regex-based replacement on the whole string
        def replace(match: re.Match[str]) -> str:
            name = match.group(1)
            return params.get(name, match.group(0))

        result = _PATH_PARAM_PATTERN.sub(replace, url)
        # Unescape any escaped colons in the result
        return result.replace("::", ":")

    # Replace only unescaped tokens and then unescape literal colons
    def replace(match: re.Match[str]) -> str:
        name = match.group(1)
        return params.get(name, match.group(0))

    new_path = _PATH_PARAM_PATTERN.sub(replace, path)
    new_path = new_path.replace("::", ":")

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


def extract_query_pairs(
    url: str, *, escape_dollars: bool = False
) -> list[tuple[str, str]]:
    """Return query-string pairs from ``url``, preserving blanks and order."""
    try:
        parsed = urlparse(url)
    except Exception:
        return []
    # QueryParam values are templates. An encoded dollar is literal URL data,
    # so escape it before decoding when the values still await substitution.
    query = parsed.query.replace("%24", "%24%24") if escape_dollars else parsed.query
    return parse_qsl(query, keep_blank_values=True)


def set_query_pairs(url: str, pairs: list[tuple[str, str]]) -> str:
    """Return ``url`` with its query string replaced by ``pairs``."""
    try:
        parsed = urlparse(url)
    except Exception:
        return url
    return urlunparse(
        (
            parsed.scheme,
            parsed.netloc,
            parsed.path,
            parsed.params,
            urlencode(pairs, doseq=True, safe="${}"),
            parsed.fragment,
        )
    )


def merge_url_query_into_params(
    url: str, params: list[tuple[str, str, bool]], *, escape_dollars: bool = False
) -> tuple[str, list[tuple[str, str, bool]]]:
    """Absorb ``url``'s query string into param triples ``(name, value, enabled)``.

    URL-bar pairs fill in empty table values for the same name. Table-only
    params are kept. The returned URL has an empty query string so callers
    can send params from one place without duplicating keys.
    """
    url_pairs = extract_query_pairs(url, escape_dollars=escape_dollars)
    if not url_pairs:
        return url, params

    # Match repeated names by occurrence rather than overwriting all their values
    # with the final table row. Unmatched table occurrences must also survive.
    table_by_name: dict[str, deque[int]] = defaultdict(deque)
    for index, (name, _, _) in enumerate(params):
        table_by_name[name].append(index)
    merged: list[tuple[str, str, bool]] = []
    consumed: set[int] = set()
    for name, value in url_pairs:
        if table_by_name[name]:
            index = table_by_name[name].popleft()
            consumed.add(index)
            _, existing_value, enabled = params[index]
            merged.append((name, existing_value if existing_value else value, enabled))
        else:
            merged.append((name, value, True))
    for index, (name, value, enabled) in enumerate(params):
        if index not in consumed:
            merged.append((name, value, enabled))

    return set_query_pairs(url, []), merged
