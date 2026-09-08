from __future__ import annotations

import logging
import os
from dataclasses import dataclass
from pathlib import Path

try:
    from dotenv import load_dotenv

    _env_file = Path(__file__).resolve().parent.parent.parent / ".env"
    if _env_file.exists():
        load_dotenv(_env_file)
except ImportError:
    pass


def _get_bool(name: str, default: bool) -> bool:
    raw = os.getenv(name)
    if raw is None:
        return default
    return raw.strip().lower() in {"1", "true", "yes", "on"}


def _get_float(name: str, default: float) -> float:
    raw = os.getenv(name)
    if raw is None:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


def _get_int(name: str, default: int) -> int:
    raw = os.getenv(name)
    if raw is None:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


@dataclass(frozen=True)
class Settings:
    app_name: str = os.getenv("APP_NAME", "ErroNotebook AI Service")
    app_version: str = os.getenv("APP_VERSION", "0.2.0")
    log_level: str = os.getenv("LOG_LEVEL", "INFO")

    ocr_backend: str = os.getenv("OCR_BACKEND", "auto")
    ocr_enable_preprocess: bool = _get_bool("OCR_ENABLE_PREPROCESS", True)
    ocr_mock_delay_seconds: float = _get_float("OCR_MOCK_DELAY_SECONDS", 0.2)
    ocr_low_confidence_threshold: float = _get_float("OCR_LOW_CONFIDENCE_THRESHOLD", 0.72)

    llm_backend: str = os.getenv("LLM_BACKEND", "mock")
    llm_mock_delay_seconds: float = _get_float("LLM_MOCK_DELAY_SECONDS", 0.2)
    openai_base_url: str = os.getenv("OPENAI_BASE_URL", "").rstrip("/")
    openai_api_key: str = os.getenv("OPENAI_API_KEY", "")
    openai_model: str = os.getenv("OPENAI_MODEL", "")
    openai_timeout_seconds: int = _get_int("OPENAI_TIMEOUT_SECONDS", 30)
    openai_temperature: float = _get_float("OPENAI_TEMPERATURE", 0.2)
    openai_max_tokens: int = _get_int("OPENAI_MAX_TOKENS", 1200)
    ai_max_attempts: int = _get_int("AI_MAX_ATTEMPTS", 3)
    ai_max_repairs: int = _get_int("AI_MAX_REPAIRS", 1)
    ai_action_timeout_seconds: int = _get_int("AI_ACTION_TIMEOUT_SECONDS", 45)
    ai_history_max_items: int = _get_int("AI_HISTORY_MAX_ITEMS", 12)
    ai_context_max_chars: int = _get_int("AI_CONTEXT_MAX_CHARS", 24000)
    ai_max_image_bytes: int = _get_int("AI_MAX_IMAGE_BYTES", 8 * 1024 * 1024)
    ai_max_image_count: int = _get_int("AI_MAX_IMAGE_COUNT", 4)

    vision_base_url: str = os.getenv("VISION_BASE_URL", "").rstrip("/")
    vision_api_key: str = os.getenv("VISION_API_KEY", "")
    vision_model: str = os.getenv("VISION_MODEL", "Qwen3-VL-32B-Instruct")
    vision_timeout_seconds: int = _get_int("VISION_TIMEOUT_SECONDS", 120)
    vision_temperature: float = _get_float("VISION_TEMPERATURE", 0.2)
    vision_max_tokens: int = _get_int("VISION_MAX_TOKENS", 2000)

    @property
    def openai_enabled(self) -> bool:
        return self.llm_backend in {"openai", "openai_compatible", "auto"} and bool(
            self.openai_base_url and self.openai_api_key and self.openai_model
        )

    @property
    def vision_enabled(self) -> bool:
        return bool(self.vision_base_url and self.vision_api_key and self.vision_model)


settings = Settings()
