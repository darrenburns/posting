import pytest
from posting.variables import find_variables, variable_range_at_cursor


@pytest.mark.parametrize(
    "text, expected",
    [
        ("", []),  # Empty string
        ("hello", []),  # No variables
        ("Hello, $name!", [("name", 7, 12)]),  # Single var
        ("Hello, $name! $age", [("name", 7, 12), ("age", 14, 18)]),  # Multiple vars
        ("Hello, $$foo $bar", [("bar", 13, 17)]),  # Escaped vars
        ("$hello", [("hello", 0, 6)]),  # Single var at start
        ("$", []),  # Empty var name
        ("$$", []),  # No variables
        ("${hello}", [("hello", 0, 8)]),  # Variable with braces
        (
            "${hello} ${world}",
            [("hello", 0, 8), ("world", 9, 17)],
        ),  # Multiple vars with braces
        ("$${hello}", []),  # Escaped braces
        ("${}", []),  # Empty braces
    ],
)
def test_find_variables(text: str, expected: list[tuple[str, int, int]]):
    assert find_variables(text) == expected


@pytest.mark.parametrize(
    "text, cursor, expected",
    [
        # Basic checks
        ("", 0, None),
        ("", 1, None),  # Out of bounds
        ("", -1, None),  # Out of bounds
        ("Hello, $name!", 0, None),
        ("Hello, $name!", 7, None),
        ("Hello, $name!", 8, (7, 12)),
        ("Hello, $name!", 9, (7, 12)),
        ("Hello, $name!", 10, (7, 12)),
        ("Hello, $name!", 11, (7, 12)),
        ("Hello, $name!", 12, None),
        ("Hello, $name!", 13, None),
        # Escaped variables
        ("Hello, $$name!", 0, None),
        ("Hello, $$name!", 7, None),
        ("Hello, $$name!", 8, None),
        ("Hello, $$name!", 9, None),
        ("Hello, $$name!", 10, None),
        ("Hello, $$name!", 11, None),
        ("Hello, $$name!", 12, None),
        # Braces
        ("${}", 0, None),
        ("${name}", 0, None),
        ("${name}", 1, (0, 7)),
        ("${name}", 2, (0, 7)),
        # Empty Braces
        ("${}", 0, None),
        ("${}", 1, None),
        ("${}", -1, None),
        ("${}", 2, None),
        ("${}", 3, None),
        # Escaped braces
        ("$${name}", 0, None),
        ("$${name}", 1, None),
        ("$${name}", 2, None),
        ("$${name}", 3, None),
        ("$${name}", 4, None),
        ("$${name}", 5, None),
    ],
)
def test_variable_range_at_cursor(
    text: str, cursor: int, expected: tuple[int, int] | None
):
    assert variable_range_at_cursor(cursor, text) == expected
