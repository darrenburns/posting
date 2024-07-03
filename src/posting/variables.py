from __future__ import annotations

import re
from string import Template

_VARIABLES_PATTERN = re.compile(
    r"\$(?:\{((?:env:)?[_a-z][_a-z0-9]*)\}|(env:[_a-z][_a-z0-9]*))"
)


class VariablesTemplate(Template):
    delimiter = "$"
    pattern = r"""
    \$(?:
      (?P<escaped>\$) |
      (?:
        \{(?P<braced>(?:env:)?[_a-z][_a-z0-9]*)\} |
        (?P<named>env:[_a-z][_a-z0-9]*)
      )
    )
    """


def find_variables(template_str: str) -> list[tuple[str, int, int]]:
    return [
        (m.group(1) or m.group(2), m.start(), m.end())
        for m in re.finditer(_VARIABLES_PATTERN, template_str)
    ]


class SubstitutionError(Exception):
    """Raised when the user refers to a variable that doesn't exist."""
