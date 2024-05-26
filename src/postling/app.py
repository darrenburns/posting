from textual.app import App, ComposeResult
from textual.widgets import Footer, Label


class Postling(App[None]):
    ENABLE_COMMAND_PALETTE = False

    def compose(self) -> ComposeResult:
        yield Label("Postling")
        yield Footer()


app = Postling()
if __name__ == "__main__":
    app.run()
