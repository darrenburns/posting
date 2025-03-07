import argparse
import base64
from datetime import datetime
import shlex
from typing import cast
from urllib.parse import ParseResult, parse_qsl, urlparse

from posting.collection import (
    Auth,
    FormItem,
    Header,
    HttpRequestMethod,
    Options,
    QueryParam,
    RequestBody,
    RequestModel,
    Scripts,
)


class CurlImport:
    """
    Parses a curl command string and extracts HTTP request components.
    """

    def __init__(self, curl_command: str):
        """
        Initialize the parser with a curl command.

        Args:
            curl_command (str): The curl command string to parse.
        """
        self.curl_command = curl_command

        # Remove leading 'curl ' if present
        if curl_command.strip().startswith("curl "):
            curl_command = curl_command.strip()[5:]

        # Replace line breaks and `\`. If we don't do this, argparse can crash when pasting requests from chrome
        curl_command = curl_command.replace("\\\n", " ")
        curl_command = curl_command.replace("\\", " ")

        # Split the command string into tokens
        tokens = shlex.split(curl_command)
        parser = argparse.ArgumentParser()
        # Define the arguments to parse
        parser.add_argument("-X", "--request", help="Specify request command to use")
        parser.add_argument(
            "-H", "--header", action="append", help="Pass custom header(s) to server"
        )
        parser.add_argument("-d", "--data", help="HTTP POST data")
        parser.add_argument("--data-raw", help="HTTP POST raw data")
        parser.add_argument("--data-binary", help="HTTP POST binary data")
        parser.add_argument(
            "-F", "--form", action="append", help="Specify multipart MIME data"
        )
        parser.add_argument("-u", "--user", help="Server user and password")
        parser.add_argument(
            "--compressed", action="store_true", help="Request compressed response"
        )
        parser.add_argument(
            "-k",
            "--insecure",
            action="store_true",
            help="Allow insecure server connections when using SSL",
        )
        parser.add_argument("-e", "--referer", help="Referrer URL")
        parser.add_argument("-A", "--user-agent", help="User-Agent to send to server")
        parser.add_argument(
            "-m", "--max-time", help="Maximum time to wait for a response"
        )
        parser.add_argument(
            "--digest",
            action="store_true",
            help="Use HTTP Digest Authentication",
        )
        parser.add_argument("url", nargs="?")

        args, extras = parser.parse_known_intermixed_args(tokens)
        # Extract components
        self.method = cast(
            HttpRequestMethod,
            args.request
            or (
                "POST"
                if args.data or args.form or args.data_raw or args.data_binary
                else "GET"
            ),
        )
        self.headers: list[tuple[str, str]] = []
        if args.header:
            for header in args.header:
                name, sep, value = header.partition(":")
                if sep:
                    self.headers.append((name.strip(), value.strip()))

        self.url = args.url
        self.user = args.user
        self.compressed = args.compressed
        self.insecure = args.insecure
        self.referer = args.referer
        self.user_agent = args.user_agent
        self.use_digest = args.digest

        # Determine if the data is form data
        self.is_form_data = False
        content_type_header = next(
            (value for name, value in self.headers if name.lower() == "content-type"),
            "",
        ).lower()

        if args.form:
            self.is_form_data = True
        elif args.data or args.data_raw or args.data_binary:
            if "application/x-www-form-urlencoded" in content_type_header:
                self.is_form_data = True
            elif not any("content-type" in h.lower() for h, _ in self.headers):
                # Default content type for -d is application/x-www-form-urlencoded
                self.is_form_data = True

        # Store raw data
        self.data = args.data or args.data_raw or args.data_binary

        # Parse data into key-value pairs if it is form data
        if self.is_form_data and self.data:
            self.data_pairs = self.parse_data(self.data)
        else:
            self.data_pairs = []  # Not form data, so no key-value pairs

        # Parse form data into key-value pairs
        if args.form:
            self.form = self.parse_form(args.form)
        else:
            self.form = []

    def parse_data(self, data_str: str) -> list[tuple[str, str]]:
        """Parse the data string into a list of tuples."""
        if not data_str:
            return []
        pairs = data_str.split("&")
        data: list[tuple[str, str]] = []
        for pair in pairs:
            if "=" in pair:
                key, value = pair.split("=", 1)
                data.append((key, value))
            else:
                data.append((pair, ""))
        return data

    def parse_form(self, form_list: list[str]) -> list[tuple[str, str]]:
        """Parse the form data into a list of tuples."""
        if not form_list:
            return []
        form_data: list[tuple[str, str]] = []
        for item in form_list:
            if "=" in item:
                key, value = item.split("=", 1)
                form_data.append((key, value))
            else:
                form_data.append((item, ""))
        return form_data

    def _extract_auth_from_headers(self) -> tuple[Auth | None, list[tuple[str, str]]]:
        """Extract authentication info from headers, returning the Auth object and remaining headers.

        Returns:
            A tuple of (Auth object or None, list of remaining headers)
        """
        remaining_headers = []
        auth: Auth | None = None

        # First check the -u/--user parameter
        if self.user:
            username, _, password = self.user.partition(":")
            # Use digest auth if --digest flag was provided, otherwise use basic auth
            if self.use_digest:
                auth = Auth.digest_auth(username, password)
            else:
                auth = Auth.basic_auth(username, password)

        # Look for auth headers that might override the -u parameter
        for name, value in self.headers:
            header_lower = name.lower()
            if header_lower == "authorization":
                parts = value.split(" ", 1)
                if len(parts) != 2:
                    # Not a valid auth header, keep it as a regular header
                    remaining_headers.append((name, value))
                    continue

                auth_type, auth_value = parts
                auth_type_lower = auth_type.lower()

                if auth_type_lower == "basic":
                    try:
                        # Basic auth is base64 encoded username:password
                        decoded = base64.b64decode(auth_value).decode()
                        username, _, password = decoded.partition(":")
                        auth = Auth.basic_auth(username, password)
                    except Exception:
                        # If we can't decode it, keep it as a header
                        remaining_headers.append((name, value))

                elif auth_type_lower == "digest":
                    # Parse digest auth parameters
                    # Example: Digest username="user", realm="realm", nonce="nonce", uri="/path"
                    try:
                        params = {}
                        for param in auth_value.split(","):
                            key, _, value = param.partition("=")
                            key = key.strip()
                            value = value.strip(' "')
                            params[key] = value

                        if "username" in params:
                            # We only need username/password for the auth model
                            auth = Auth.digest_auth(
                                username=params["username"],
                                password=params.get("password", ""),
                            )
                    except Exception:
                        # If we can't parse it, keep it as a header
                        remaining_headers.append((name, value))

                elif auth_type_lower == "bearer":
                    # Bearer token auth
                    try:
                        auth = Auth.bearer_token_auth(auth_value)
                    except Exception:
                        # If we can't parse it, keep it as a header
                        remaining_headers.append((name, value))

                else:
                    # Unknown auth type, keep as header
                    remaining_headers.append((name, value))
            else:
                remaining_headers.append((name, value))

        return auth, remaining_headers

    def to_request_model(self) -> RequestModel:
        """Convert the parsed curl command into a RequestModel."""
        # Parse URL and extract query parameters
        parsed_url: ParseResult = urlparse(self.url)
        base_url = f"{parsed_url.scheme}://{parsed_url.netloc}{parsed_url.path}"
        query_params = [
            QueryParam(name=name, value=value)
            for name, value in parse_qsl(parsed_url.query)
        ]

        # Extract auth and get remaining headers
        auth, remaining_headers = self._extract_auth_from_headers()

        # Build the request body if one exists
        body: RequestBody | None = None
        if self.data or self.form:
            if self.is_form_data:
                # Use form data pairs from either -F or -d
                form_data = self.form or self.data_pairs
                body = RequestBody(
                    form_data=[
                        FormItem(name=name, value=value) for name, value in form_data
                    ]
                )
            else:
                # Raw body content
                body = RequestBody(content=self.data)

        # Convert remaining headers to Header objects
        headers = [Header(name=name, value=value) for name, value in remaining_headers]

        # Set options, including the insecure flag
        options = Options(
            verify_ssl=not self.insecure,  # Invert insecure flag for verify_ssl
            follow_redirects=True,  # Default to following redirects
            attach_cookies=True,  # Default to attaching cookies
        )

        curl_command = self.curl_command.strip()
        description = f"Imported from curl at {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}:\n{curl_command}"
        return RequestModel(
            method=self.method,
            url=base_url,  # Use the URL without query parameters
            headers=headers,
            body=body,
            name=base_url,
            description=description,
            params=query_params,
            options=options,
            auth=auth,
            scripts=Scripts(),
        )
