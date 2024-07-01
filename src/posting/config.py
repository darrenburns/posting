from pydantic_settings import BaseSettings, SettingsConfigDict

from posting.types import PostingLayout


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        env_prefix="posting_",
        env_ignore_empty=True,
    )

    theme: str = "posting"
    layout: PostingLayout = "vertical"
