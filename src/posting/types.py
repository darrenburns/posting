from typing import Literal, Optional, Tuple, Union

PostingLayout = Literal["horizontal", "vertical"]

# From httpx - seems to not be public.
CertTypes = Union[
    # certfile
    str,
    # (certfile, keyfile)
    Tuple[str, Optional[str]],
    # (certfile, keyfile, password)
    Tuple[str, Optional[str], Optional[str]],
]
