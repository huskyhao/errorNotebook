"""Run the offline test suite with a deterministic default provider."""

from __future__ import annotations

import os
import sys
import unittest
from pathlib import Path


SERVICE_ROOT = Path(__file__).resolve().parent
sys.path.insert(0, str(SERVICE_ROOT))
os.environ.setdefault("LLM_BACKEND", "mock")


if __name__ == "__main__":
    suite = unittest.defaultTestLoader.discover(str(SERVICE_ROOT / "tests"))
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    raise SystemExit(not result.wasSuccessful())
