import argparse
import shlex


class CurlImport:
    def __init__(self, curl_command):
        # Remove leading 'curl ' if present
        if curl_command.strip().startswith("curl "):
            curl_command = curl_command.strip()[5:]

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
        parser.add_argument("url", nargs="?")
        # Parse the arguments
        args = parser.parse_intermixed_args(tokens)
        # Extract components
        self.method = args.request or (
            "POST"
            if args.data or args.form or args.data_raw or args.data_binary
            else "GET"
        )
        self.headers = (
            [header.split(": ") for header in args.header] if args.header else []
        )
        self.url = args.url
        self.user = args.user
        self.compressed = args.compressed
        self.insecure = args.insecure
        self.referer = args.referer
        self.user_agent = args.user_agent

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

    def parse_data(self, data_str):
        """Parse the data string into a list of tuples."""
        if not data_str:
            return []
        pairs = data_str.split("&")
        data = []
        for pair in pairs:
            if "=" in pair:
                key, value = pair.split("=", 1)
                data.append((key, value))
            else:
                data.append((pair, ""))
        return data

    def parse_form(self, form_list):
        """Parse the form data into a list of tuples."""
        if not form_list:
            return []
        form_data = []
        for item in form_list:
            if "=" in item:
                key, value = item.split("=", 1)
                form_data.append((key, value))
            else:
                form_data.append((item, ""))
        return form_data

    def as_dict(self):
        return {
            "method": self.method,
            "url": self.url,
            "headers": self.headers,
            "data": self.data,
            "data_pairs": self.data_pairs,
            "form": self.form,
            "user": self.user,
            "compressed": self.compressed,
            "insecure": self.insecure,
            "referer": self.referer,
            "user_agent": self.user_agent,
            "is_form_data": self.is_form_data,
        }
