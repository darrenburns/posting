import asyncio
import json
import os
import shutil
import ssl
import subprocess
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from unittest.mock import patch

import pytest
from test_snapshots import CONFIG_DIR
from textual.lazy import Lazy

from posting.__main__ import make_posting
from posting.collection import Options, RequestModel


@pytest.fixture(autouse=True)
def isolated_tls_environment(monkeypatch):
    monkeypatch.setenv("NO_PROXY", "127.0.0.1,localhost")
    monkeypatch.delenv("SSL_CERT_FILE", raising=False)
    monkeypatch.delenv("SSL_CERT_DIR", raising=False)


@pytest.fixture(scope="module")
def certificates(tmp_path_factory):
    openssl = shutil.which("openssl")
    if openssl is None:
        pytest.skip("OpenSSL is required to generate TLS test certificates")
    directory = tmp_path_factory.mktemp("tls")

    def run(*arguments):
        subprocess.run(
            [openssl, *arguments], cwd=directory, check=True, capture_output=True
        )

    run(
        "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
        "-subj", "/CN=Posting test CA", "-keyout", "ca.key", "-out", "ca.pem",
    )
    for name, extensions in (
        ("server", "subjectAltName=IP:127.0.0.1\nextendedKeyUsage=serverAuth\n"),
        ("client", "extendedKeyUsage=clientAuth\n"),
    ):
        run(
            "req", "-newkey", "rsa:2048", "-nodes", "-subj",
            f"/CN=posting-test-{name}", "-keyout", f"{name}.key",
            "-out", f"{name}.csr",
        )
        (directory / f"{name}.ext").write_text(extensions)
        run(
            "x509", "-req", "-in", f"{name}.csr", "-CA", "ca.pem",
            "-CAkey", "ca.key", "-CAcreateserial", "-days", "1",
            "-extfile", f"{name}.ext", "-out", f"{name}.pem",
        )
    (directory / "combined.pem").write_bytes(
        (directory / "client.pem").read_bytes()
        + (directory / "client.key").read_bytes()
    )
    run(
        "pkey", "-in", "client.key", "-aes256", "-passout",
        "pass:posting-test-password", "-out", "encrypted.key",
    )
    (directory / "encrypted-combined.pem").write_bytes(
        (directory / "client.pem").read_bytes()
        + (directory / "encrypted.key").read_bytes()
    )
    ca_directory = directory / "ca-directory"
    ca_directory.mkdir()
    shutil.copyfile(directory / "ca.pem", ca_directory / "ca.pem")
    run("rehash", str(ca_directory))
    return directory


@pytest.fixture
def tls_server(certificates):
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            certificate = self.connection.getpeercert()
            body = json.dumps(certificate).encode()
            self.send_response(200 if certificate else 401)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *args):
            pass

    context = ssl.create_default_context(ssl.Purpose.CLIENT_AUTH)
    context.load_cert_chain(certificates / "server.pem", certificates / "server.key")
    context.load_verify_locations(certificates / "ca.pem")
    context.verify_mode = ssl.CERT_OPTIONAL
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.socket = context.wrap_socket(server.socket, server_side=True)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"https://127.0.0.1:{server.server_port}/"
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


def send_request(tmp_path, url, ca_bundle=None, certificate=None, verify_ssl=True):
    config = (CONFIG_DIR / "general.yaml").read_text()
    ssl_config = {
        "ca_bundle": ca_bundle,
        "certificate_path": None,
        "key_file": None,
        "password": None,
    }
    if certificate is not None:
        path, key, password = certificate
        ssl_config.update(
            certificate_path=str(path),
            key_file=str(key) if key is not None else None,
            password=password,
        )
    # JSON is valid YAML; use the real settings loader, including secret passwords.
    config_path = tmp_path / "config.yaml"
    config_path.write_text(config + "\nssl: " + json.dumps(ssl_config) + "\n")

    async def send():
        with patch.dict(os.environ, {"POSTING_CONFIG_FILE": str(config_path)}):
            app = make_posting(tmp_path)
        async with app.run_test() as pilot:
            async with asyncio.timeout(30):
                while app.query(Lazy):
                    await pilot.pause(0.01)
            await pilot.pause()
            screen = app.screen
            screen.load_request_model(
                RequestModel(url=url, options=Options(verify_ssl=verify_ssl))
            )
            await pilot.pause()
            await screen.send_request()
            await pilot.pause()
            return screen.response_area.response, screen.url_input.has_class("error")

    return asyncio.run(send())


def assert_client_certificate(response):
    assert response is not None
    assert response.status_code == 200
    subject = response.json()["subject"]
    assert ["commonName", "posting-test-client"] in [
        attribute for group in subject for attribute in group
    ]


@pytest.mark.parametrize("ca_kind", ["file", "directory"])
@pytest.mark.parametrize(
    "certificate_kind", ["separate", "combined", "encrypted", "encrypted_combined"]
)
def test_client_certificate_with_custom_ca(
    tmp_path, certificates, tls_server, ca_kind, certificate_kind
):
    ca = certificates / ("ca.pem" if ca_kind == "file" else "ca-directory")
    certificate = {
        "separate": (certificates / "client.pem", certificates / "client.key", None),
        "combined": (certificates / "combined.pem", None, None),
        "encrypted": (
            certificates / "client.pem", certificates / "encrypted.key",
            "posting-test-password",
        ),
        "encrypted_combined": (
            certificates / "encrypted-combined.pem", None, "posting-test-password",
        ),
    }[certificate_kind]
    response, error = send_request(tmp_path, tls_server, str(ca), certificate)
    assert not error
    assert_client_certificate(response)


def test_custom_ca_without_client_certificate(tmp_path, certificates, tls_server):
    response, error = send_request(tmp_path, tls_server, str(certificates / "ca.pem"))
    assert not error
    assert response.status_code == 401


def test_client_certificate_with_verification_disabled(tmp_path, certificates, tls_server):
    response, error = send_request(
        tmp_path,
        tls_server,
        ca_bundle=str(certificates / "missing-ca.pem"),
        certificate=(certificates / "client.pem", certificates / "client.key", None),
        verify_ssl=False,
    )
    assert not error
    assert_client_certificate(response)


def test_client_certificate_with_environment_ca(
    tmp_path, certificates, tls_server, monkeypatch
):
    monkeypatch.setenv("SSL_CERT_FILE", str(certificates / "ca.pem"))
    response, error = send_request(
        tmp_path,
        tls_server,
        certificate=(certificates / "client.pem", certificates / "client.key", None),
    )
    assert not error
    assert_client_certificate(response)


def test_untrusted_server_is_rejected(tmp_path, tls_server, monkeypatch):
    monkeypatch.delenv("SSL_CERT_FILE", raising=False)
    monkeypatch.delenv("SSL_CERT_DIR", raising=False)
    response, error = send_request(tmp_path, tls_server)
    assert response is None
    assert error


def test_encrypted_combined_certificate_with_environment_ca(
    tmp_path, certificates, tls_server, monkeypatch
):
    monkeypatch.setenv("SSL_CERT_FILE", str(certificates / "ca.pem"))
    response, error = send_request(
        tmp_path,
        tls_server,
        certificate=(
            certificates / "encrypted-combined.pem", None, "posting-test-password",
        ),
    )
    assert not error
    assert_client_certificate(response)


def test_custom_ca_still_checks_hostname(tmp_path, certificates, tls_server):
    response, error = send_request(
        tmp_path,
        tls_server.replace("127.0.0.1", "localhost"),
        ca_bundle=str(certificates / "ca.pem"),
        certificate=(certificates / "client.pem", certificates / "client.key", None),
    )
    assert response is None
    assert error
