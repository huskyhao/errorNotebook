from __future__ import annotations

from pydantic import BaseModel


class HealthResponse(BaseModel):
    status: str
    ocrBackend: str
    llmBackend: str
    llmConfigured: bool
    llmModel: str | None = None
    visionBackend: str
    visionConfigured: bool
    visionModel: str | None = None
