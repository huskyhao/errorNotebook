from __future__ import annotations

import json

from fastapi import APIRouter, HTTPException, Request, status

from app.schemas.analysis import AnalysisRequest, AnalysisResponse
from app.services.analysis_service import AnalysisService
from app.services.openai_client import OpenAICompatibleError

router = APIRouter()
analysis_service = AnalysisService()


@router.post("/analyze/question", response_model=AnalysisResponse)
async def analyze_question(request: Request) -> AnalysisResponse:
    content_type = request.headers.get("content-type", "")
    if "multipart/form-data" not in content_type:
        raise HTTPException(status_code=status.HTTP_415_UNSUPPORTED_MEDIA_TYPE,
                            detail="Expected multipart/form-data")

    form = await request.form()

    payload_str = form.get("payload")
    if payload_str is None:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST,
                            detail="Missing 'payload' field")
    try:
        payload = AnalysisRequest.model_validate(json.loads(str(payload_str)))
    except (json.JSONDecodeError, ValueError) as exc:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"invalid payload JSON: {exc}",
        ) from exc

    image_bytes: bytes | None = None
    media_type: str | None = None
    file_field = form.get("file")
    # Swagger UI sends empty string when no file selected; skip it.
    if file_field is not None and hasattr(file_field, "filename") and getattr(file_field, "filename"):
        image_bytes = await file_field.read()
        media_type = getattr(file_field, "content_type", "image/png") or "image/png"

    try:
        return await analysis_service.analyze_question(
            payload, image_bytes=image_bytes, media_type=media_type
        )
    except OpenAICompatibleError as exc:
        raise HTTPException(
            status_code=status.HTTP_502_BAD_GATEWAY,
            detail=str(exc),
        ) from exc
