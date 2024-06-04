from posting.datatable import PostingDataTable


class CookiesTable(PostingDataTable):
    def on_mount(self) -> None:
        self.show_header = False
        self.cursor_type = "row"
        self.zebra_stripes = True
        self.add_columns(*["Header", "Value"])

