from __future__ import annotations

import uuid

from fastapi import APIRouter, File, Form, HTTPException, UploadFile, status

from app.schemas.analysis import AnalysisRequest, AnalysisResponse
from app.schemas.ocr import OCRResponse
from app.schemas.question import QuestionMetadata, StructuredQuestion
from app.services.analysis_service import AnalysisService
from app.services.ocr_service import OCRService
from app.services.openai_client import OpenAICompatibleError

router = APIRouter(prefix="/debug")
ocr_service = OCRService()
analysis_service = AnalysisService()


@router.post("/ocr", response_model=OCRResponse)
async def debug_ocr(
    file: UploadFile = File(...),
    question_id: int = Form(default=10001),
    source_type: str = Form(default="image"),
    trace_id: str | None = Form(default=None),
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

    return await ocr_service.parse_document(
        question_id=question_id,
        filename=file.filename or "unknown",
        source_type=source_type,
        trace_id=trace_id or uuid.uuid4().hex,
        media_type=file.content_type,
        file_bytes=file_bytes,
    )


@router.post("/analyze/mock-question", response_model=AnalysisResponse)
async def debug_analyze_mock_question(
    trace_id: str | None = Form(default=None),
    question_id: int = Form(default=10001),
    stem: str = Form(default="已知函数 f(x)=x^2-4x+3，求其最小值。"),
    user_answer: str | None = Form(default="1"),
) -> AnalysisResponse:
    payload = AnalysisRequest(
        traceId=trace_id or uuid.uuid4().hex,
        questionId=question_id,
        userAnswer=user_answer,
        context={"debug": True, "source": "debug.mock-question"},
        question=StructuredQuestion(
            stem=stem,
            questionType="subjective",
            options=[],
            assets=[],
            suggestedAnswer="1",
            rawText=stem,
            hasDiagram=False,
            warnings=[],
            metadata=QuestionMetadata(
                sourceType="debug",
                hasDiagram=False,
                ocrConfidence=1.0,
                importMode="single_question",
                extractionMethod="debug_mock",
            ),
        ),
    )

    try:
        return await analysis_service.analyze_question(payload)
    except OpenAICompatibleError as exc:
        raise HTTPException(status_code=status.HTTP_502_BAD_GATEWAY, detail=str(exc)) from exc
