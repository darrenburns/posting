from contextvars import ContextVar
from typing import Type
from pydantic import BaseModel, Field
from pydantic_settings import (
    BaseSettings,
    PydanticBaseSettingsSource,
    SettingsConfigDict,
    YamlConfigSettingsSource,
)
from textual.types import AnimationLevel
import yaml

from posting.locations import config_file

from posting.types import PostingLayout


class HeadingSettings(BaseModel):
    visible: bool = Field(default=True)
    """Whether this widget should be displayed or not."""
    show_host: bool = Field(default=True)
    """Whether or not to show the hostname in the app header."""


class ResponseSettings(BaseModel):
    """Configuration for the response viewer."""

    prettify_json: bool = Field(default=True)
    """If enabled, JSON responses will be pretty-formatted."""


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        env_prefix="posting_",
        env_nested_delimiter="__",
        env_ignore_empty=True,
        extra="allow",
    )

    theme: str = Field(default="posting")
    """The name of the theme to use."""

    layout: PostingLayout = Field(default="vertical")
    """Layout for the app."""

    use_host_environment: bool = Field(default=False)
    """If enabled, you can use environment variables from the host machine in your requests 
    using the `${env:VARIABLE_NAME}` syntax. When disabled, you are restricted to variables
    defined in any `.env` files explicitly supplied via the `--env` option."""

    animation: AnimationLevel = Field(default="none")
    """Controls the amount of animation permitted."""

    response: ResponseSettings = Field(default_factory=ResponseSettings)
    """Configuration for the response viewer."""

    heading: HeadingSettings = Field(default_factory=HeadingSettings)
    """Configuration for the heading bar."""

    @classmethod
    def settings_customise_sources(
        cls,
        settings_cls: Type[BaseSettings],
        init_settings: PydanticBaseSettingsSource,
        env_settings: PydanticBaseSettingsSource,
        dotenv_settings: PydanticBaseSettingsSource,
        file_secret_settings: PydanticBaseSettingsSource,
    ) -> tuple[PydanticBaseSettingsSource, ...]:
        conf_file = config_file()
        default_sources = (
            init_settings,
            env_settings,
            dotenv_settings,
            file_secret_settings,
        )

        # TODO - this is working around a crash in pydantic-settings
        # where the yaml settings source seems to crash if the file
        # is empty.
        # This workaround conditionally loads the yaml config file.
        # If it's empty, we don't use it.
        # https://github.com/pydantic/pydantic-settings/issues/329
        try:
            yaml_config = yaml.load(conf_file.read_bytes(), Loader=yaml.Loader)
        except yaml.YAMLError:
            return default_sources

        if conf_file.exists() and yaml_config:
            return (
                init_settings,
                YamlConfigSettingsSource(settings_cls, conf_file),
                env_settings,
                dotenv_settings,
                file_secret_settings,
            )
        return default_sources


SETTINGS: ContextVar[Settings] = ContextVar("settings")
