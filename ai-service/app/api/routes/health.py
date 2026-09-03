from __future__ import annotations

from fastapi import APIRouter

from app.core.config import settings
from app.schemas.common import HealthResponse

router = APIRouter()


@router.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    return HealthResponse(
        status="ok",
        ocrBackend=settings.ocr_backend,
        llmBackend="openai_compatible" if settings.openai_enabled else settings.llm_backend,
    )
