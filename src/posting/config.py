from contextvars import ContextVar
import os
from pathlib import Path
from typing import Literal, Type
from pydantic import BaseModel, Field, SecretStr
from pydantic_settings import (
    BaseSettings,
    PydanticBaseSettingsSource,
    SettingsConfigDict,
    YamlConfigSettingsSource,
)
from textual.types import AnimationLevel

from posting.locations import config_file, theme_directory

from posting.types import PostingLayout


class HeadingSettings(BaseModel):
    visible: bool = Field(default=True)
    """Whether this widget should be displayed or not."""
    show_host: bool = Field(default=True)
    """Whether or not to show the hostname in the app header."""
    show_version: bool = Field(default=True)
    """Whether or not to show the version in the app header."""
    hostname: str | None = Field(default=None)
    """The hostname to display in the app header.

    You may use Rich markup here.
    
    If unset, the hostname provided via `socket.gethostname()` will be used.
    """


class UrlBarSettings(BaseModel):
    show_value_preview: bool = Field(default=True)
    """If enabled, the variable value bar will be displayed below the URL.

    When your cursor is above a variable, the value will be displayed on
    the line below the URL bar."""


class ResponseSettings(BaseModel):
    """Configuration for the response viewer."""

    prettify_json: bool = Field(default=True)
    """If enabled, JSON responses will be pretty-formatted."""

    show_size_and_time: bool = Field(default=True)
    """If enabled, the size and time taken for the response will be displayed."""


class FocusSettings(BaseModel):
    """Configuration relating to focus."""

    on_startup: Literal["url", "method", "collection"] = Field(default="url")
    """On startup, move focus to the URL bar, method, or collection browser."""

    on_response: Literal["body", "tabs"] | None = Field(default=None)
    """On receiving a response, move focus to the body or the response section (the tabs).

    If this value is unset, focus will not shift when a response is received."""

    on_request_open: (
        Literal["headers", "body", "query", "info", "url", "method"] | None
    ) = Field(default=None)
    """On opening a request using the sidebar collection browser, move focus to the specified target.

    Valid values are: `headers`, `body`, `query`, `info`, `url`, `method`.

    This will move focus *inside* the target tab, to the topmost widget in the tab.
    
    If this value is unset, focus will not shift when a request is opened."""


class CertificateSettings(BaseModel):
    """Configuration for SSL CA bundles and client certificates."""

    ca_bundle: str | None = Field(default=None)
    """Absolute path to the CA bundle file."""
    certificate_path: str | None = Field(default=None)
    """Absolute path to the client certificate .pem file or directory"""
    key_file: str | None = Field(default=None)
    """Absolute path to the key file"""
    password: SecretStr | None = Field(default=None)
    """Password for the key file."""


class TextInputSettings(BaseModel):
    """Configuration for text input widgets."""

    blinking_cursor: bool = Field(default=True)
    """If enabled, the cursor will blink in input widgets and text areas."""


class CommandPaletteSettings(BaseModel):
    """Configuration for the command palette."""

    theme_preview: bool = Field(default=False)
    """If enabled, the command palette will display a preview of the selected theme when the cursor is over it."""


class CollectionBrowserSettings(BaseModel):
    """Configuration for the collection browser."""

    position: Literal["left", "right"] = Field(default="left")
    """The position of the collection browser on screen."""

    show_on_startup: bool = Field(default=True)
    """If enabled, the collection browser will be shown on startup."""


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        env_prefix="posting_",
        env_nested_delimiter="__",
        env_ignore_empty=True,
        extra="allow",
    )

    theme: str = Field(default="galaxy")
    """The name of the theme to use."""

    theme_directory: Path = Field(default=theme_directory())
    """The directory containing user themes."""

    load_user_themes: bool = Field(default=True)
    """If enabled, load user themes from the theme directory, allowing them
    to be specified in config and selected via the command palette."""

    load_builtin_themes: bool = Field(default=True)
    """If enabled, load builtin themes, allowing them to be specified
    in config and selected via the command palette."""

    layout: PostingLayout = Field(default="vertical")
    """Layout for the app."""

    use_host_environment: bool = Field(default=False)
    """If enabled, you can use environment variables from the host machine in your requests
    using the `${VARIABLE_NAME}` syntax. When disabled, you are restricted to variables
    defined in any `.env` files explicitly supplied via the `--env` option."""

    watch_env_files: bool = Field(default=True)
    """If enabled, automatically reload environment files when they change."""

    watch_collection_files: bool = Field(default=True)
    """If enabled, automatically reload collection files when they change."""

    watch_themes: bool = Field(default=True)
    """If enabled, automatically reload themes in the theme directory when they change on disk."""

    text_input: TextInputSettings = Field(default_factory=TextInputSettings)
    """General configuration for inputs and text area widgets."""

    animation: AnimationLevel = Field(default="none")
    """Controls the amount of animation permitted."""

    response: ResponseSettings = Field(default_factory=ResponseSettings)
    """Configuration for the response viewer."""

    heading: HeadingSettings = Field(default_factory=HeadingSettings)
    """Configuration for the heading bar."""

    url_bar: UrlBarSettings = Field(default_factory=UrlBarSettings)
    """Configuration for the URL bar."""

    collection_browser: CollectionBrowserSettings = Field(
        default_factory=CollectionBrowserSettings
    )
    """Configuration for the collection browser."""

    command_palette: CommandPaletteSettings = Field(
        default_factory=CommandPaletteSettings
    )
    """Configuration for the command palette."""

    pager: str | None = Field(default=os.getenv("PAGER"))
    """The command to use for paging."""

    pager_json: str | None = Field(default=None)
    """The command to use for paging JSON.

    This will be used when the pager is opened from within a TextArea,
    and the content within that TextArea can be inferred to be JSON.

    For example, the editor is set to JSON language, or the response content
    type indicates JSON.

    If this is unset, the standard `pager` config will be used.
    """

    editor: str | None = Field(default=os.getenv("EDITOR"))
    """The command to use for editing."""

    use_xresources: bool = Field(default=False)
    """If true, try to use Xresources to create dark and light themes."""

    ssl: CertificateSettings = Field(default_factory=CertificateSettings)
    """Configuration for SSL CA bundle and client certificates."""

    focus: FocusSettings = Field(default_factory=FocusSettings)
    """Configuration for focus."""

    keymap: dict[str, str] = Field(default_factory=dict)
    """A dictionary mapping binding IDs to key combinations."""

    curl_export_extra_args: str = Field(default="")
    """Extra arguments to pass to curl when exporting a request as a curl command."""

    @classmethod
    def settings_customise_sources(
        cls,
        settings_cls: Type[BaseSettings],
        init_settings: PydanticBaseSettingsSource,
        env_settings: PydanticBaseSettingsSource,
        dotenv_settings: PydanticBaseSettingsSource,
        file_secret_settings: PydanticBaseSettingsSource,
    ) -> tuple[PydanticBaseSettingsSource, ...]:
        config_from_env = os.getenv("POSTING_CONFIG_FILE")
        if config_from_env:
            conf_file = Path(config_from_env).resolve()
        else:
            conf_file = config_file()

        default_sources = (
            init_settings,
            env_settings,
            dotenv_settings,
            file_secret_settings,
        )

        if conf_file.exists():
            return (
                init_settings,
                YamlConfigSettingsSource(settings_cls, conf_file),
                env_settings,
                dotenv_settings,
                file_secret_settings,
            )
        return default_sources


SETTINGS: ContextVar[Settings] = ContextVar("settings")
