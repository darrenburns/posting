from typing import Literal
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        env_prefix="posting_",
        env_ignore_empty=True,
    )

    theme: str = "posting"
    layout: Literal["vertical", "horizontal"] = "vertical"
