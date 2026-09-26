from posting.themes import Theme


class TestTextAreaThemeFromThemeVariables:
    def test_handles_ansi_prefixed_colors_without_raising(self) -> None:
        """Regression test for #365: Textual's builtin ansi-dark/ansi-light themes express
        every variable using the `ansi_<name>` convention (e.g. "ansi_green"), which
        rich.style.Style.parse doesn't understand and previously raised StyleSyntaxError.
        Values below are taken from the ansi-dark theme's actual variables, as shown in the
        traceback attached to #365.
        """
        variables = {
            "text-accent": "ansi_green",
            "text-secondary": "ansi_cyan",
            "text-success": "ansi_green",
            "text-warning": "ansi_yellow",
            "text-primary": "ansi_default",
            "text-area-gutter": "ansi_black",
            "text-area-cursor": "ansi_white",
        }

        text_area_theme = Theme.text_area_theme_from_theme_variables(variables)

        assert text_area_theme.syntax_styles["string"].color is not None
        assert text_area_theme.gutter_style is not None

    def test_still_handles_ordinary_hex_colors(self) -> None:
        """Non-ansi themes (the common case: hex colors) keep working unchanged."""
        variables = {
            "text-accent": "#C45AFF",
            "text-secondary": "#a684e8",
            "text-success": "#00FA9A",
            "text-warning": "#FFD700",
            "text-primary": "#FFFFFF",
        }

        text_area_theme = Theme.text_area_theme_from_theme_variables(variables)

        assert text_area_theme.syntax_styles["string"].color is not None

    def test_still_handles_compound_style_strings(self) -> None:
        """Custom themes can set a full Rich style spec (foreground + background), not just
        a bare color - only Style.parse understands that syntax, so the #365 fallback to
        Color.parse must not run for these.
        """
        variables = {
            "text-accent": "#C45AFF",
            "text-secondary": "#a684e8",
            "text-success": "#00FA9A",
            "text-warning": "#FFD700",
            "text-primary": "#FFFFFF",
            "text-area-matched-bracket": "black on red",
        }

        text_area_theme = Theme.text_area_theme_from_theme_variables(variables)

        assert text_area_theme.bracket_matching_style is not None
        assert text_area_theme.bracket_matching_style.color is not None
        assert text_area_theme.bracket_matching_style.bgcolor is not None
