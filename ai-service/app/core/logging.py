from __future__ import annotations

import logging
import os
from datetime import datetime, timezone
from pathlib import Path


_LOG_DIR = Path(__file__).resolve().parent.parent / ".log"


def setup_logging() -> None:
    level_name = os.getenv("LOG_LEVEL", "INFO").upper()
    level = getattr(logging, level_name, logging.INFO)

    _LOG_DIR.mkdir(parents=True, exist_ok=True)
    timestamp = datetime.now(timezone.utc).strftime("%Y%m%d_%H%M%S")
    log_path = _LOG_DIR / f"{timestamp}.log"

    logging.basicConfig(
        level=level,
        format="%(asctime)s | %(levelname)s | %(name)s | %(message)s",
        handlers=[
            logging.StreamHandler(),
            logging.FileHandler(log_path, encoding="utf-8"),
        ],
        # Uvicorn installs handlers before importing the application. Without
        # force=True, basicConfig becomes a no-op and runtime requests only
        # appear in the launch terminal instead of the intended log file.
        force=True,
    )


def format_log(event: str, **fields: object) -> str:
    parts = [f"event={event}"]
    for key, value in fields.items():
        parts.append(f"{key}={value!r}")
    return " ".join(parts)
