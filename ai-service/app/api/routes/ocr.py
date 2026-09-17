from __future__ import annotations

import uuid

from fastapi import APIRouter, File, Form, HTTPException, UploadFile, status

from app.schemas.ocr import OCRResponse
from app.services.ai_errors import AIServiceError
from app.services.ocr_service import OCRService

router = APIRouter()
ocr_service = OCRService()


@router.post("/ocr/parse", response_model=OCRResponse)
async def parse_ocr(
    question_id: int = Form(...),
    source_type: str = Form(default="image"),
    trace_id: str | None = Form(default=None),
    file: UploadFile = File(...),
) -> OCRResponse:
    if not file.content_type:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="file content type is required")

    if not file.content_type.startswith("image/"):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="only image upload is supported",
        )

    file_bytes = await file.read()
    if not file_bytes:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="file is empty")

    request_trace_id = trace_id or uuid.uuid4().hex
    try:
        return await ocr_service.parse_document(
            question_id=question_id,
            filename=file.filename or "unknown",
            source_type=source_type,
            trace_id=request_trace_id,
            media_type=file.content_type,
            file_bytes=file_bytes,
        )
    except AIServiceError as exc:
        raise HTTPException(
            status_code=exc.status_code,
            detail={
                "code": exc.code,
                "message": exc.message,
                "retryable": exc.retryable,
                "traceId": request_trace_id,
            },
        ) from exc
