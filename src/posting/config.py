from typing import Type
from pydantic import Field
from pydantic_settings import (
    BaseSettings,
    PydanticBaseSettingsSource,
    SettingsConfigDict,
    YamlConfigSettingsSource,
)
import yaml

from posting.locations import config_file

from posting.types import PostingLayout


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        env_prefix="posting_",
        env_ignore_empty=True,
        extra="allow",
    )

    theme: str = Field(default="posting")
    layout: PostingLayout = Field(default="vertical")

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
