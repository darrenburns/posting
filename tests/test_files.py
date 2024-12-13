import pytest
from posting.files import is_valid_filename


@pytest.mark.parametrize(
    "filename, expected",
    [
        ("valid_filename.txt", True),
        ("", False),
        ("   ", False),
        ("file/with/path.txt", False),
        ("a" * 255, True),
        ("a" * 256, False),
        ("CON", False),
        ("PRN.txt", False),
        ("AUX.log", False),
        ("NUL.dat", False),
        ("COM1.bin", False),
        ("LPT1.tmp", False),
        ("file..with..dots.txt", False),
        (".hidden_file.txt", False),
        ("normal.file.txt", True),
        ("file-with-dashes.txt", True),
        ("file_with_underscores.txt", True),
        ("file with spaces.txt", True),
        ("file.with.multiple.extensions.txt", True),
        # Path traversal attack tests
        ("../filename.txt", False),
        ("filename/../something.txt", False),
        ("foo/../bar/baz.txt", False),
        ("foo/./bar/baz.txt", False),
        # Absolute path tests
        ("/etc/passwd", False),
        ("/var/log/system.log", False),
        ("/home/user/file.txt", False),
        ("/file.txt", False),
        ("C:/Program Files/file.txt", False),
    ],
)
def test_is_valid_filename(filename, expected):
    assert is_valid_filename(filename) == expected


def test_is_valid_filename_with_none():
    assert is_valid_filename(None) is False


@pytest.mark.parametrize(
    "os_specific_filename, expected",
    [
        ("COM0", True),  # COM0 is not in the reserved list
        ("LPT0", True),  # LPT0 is not in the reserved list
        ("CON.txt", False),
        ("AUX.log", False),
    ],
)
def test_is_valid_filename_os_specific(os_specific_filename, expected):
    assert is_valid_filename(os_specific_filename) == expected
