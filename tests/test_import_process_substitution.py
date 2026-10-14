import json
import os
from pathlib import Path

import pytest
from click.testing import CliRunner

from posting.__main__ import cli

SPEC = {
    "openapi": "3.1.0",
    "info": {"title": "Test", "version": "1.0"},
    "servers": [{"url": "https://api.example.com"}],
    "paths": {"/ping": {"get": {"responses": {"200": {"description": "OK"}}}}},
}


@pytest.mark.skipif(not Path("/dev/fd").is_dir(), reason="requires /dev/fd")
def test_import_openapi_from_pipe(tmp_path: Path):
    """posting import <(command) passes a pipe such as /dev/fd/63.

    Its parent directory is not a place to write the environment files, so
    they must go into the output directory with the rest of the collection.
    """
    read_fd, write_fd = os.pipe()
    try:
        os.write(write_fd, json.dumps(SPEC).encode())
        os.close(write_fd)
        output = tmp_path / "collection"

        result = CliRunner().invoke(
            cli, ["import", f"/dev/fd/{read_fd}", "-o", str(output)]
        )
    finally:
        os.close(read_fd)

    assert "Successfully imported" in result.output
    assert "Traceback" not in result.output
    assert (output / "ping.posting.yaml").exists()
    assert (output / "api_example_com.env").exists()
