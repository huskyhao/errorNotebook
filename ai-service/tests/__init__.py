"""Test-package defaults that keep the suite offline and deterministic."""

import os


os.environ.setdefault("LLM_BACKEND", "mock")
