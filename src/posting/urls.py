import re

def ensure_protocol(url: str) -> str:
    """Default the protocol to http:// if no protocol is present.
    
    Args:
        url: The URL to ensure has a protocol.

    Returns:
        The URL with a the http:// protocol if no protocol was present, 
        otherwise the original URL.
    """
    if re.match(r'^[a-zA-Z][a-zA-Z0-9+.-]*://', url):
        return url
    return f"http://{url}"