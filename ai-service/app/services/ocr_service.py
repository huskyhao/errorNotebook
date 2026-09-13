from __future__ import annotations

import asyncio
import logging
import time

from app.core.config import settings
from app.core.logging import format_log
from app.schemas.ocr import OCRResponse
from app.services.ocr_backends import build_ocr_backend
from app.services.question_structure_refiner import QuestionStructureRefiner
from app.services.question_structurer import QuestionStructurer

logger = logging.getLogger("app.services.ocr")


class OCRService:
    def __init__(self) -> None:
        self._backend = build_ocr_backend()
        self._structurer = QuestionStructurer()
        self._refiner = QuestionStructureRefiner()

    @property
    def backend_name(self) -> str:
        return self._backend.__class__.__name__.replace("Backend", "").lower()

    async def parse_document(
        self,
        *,
        question_id: int,
        filename: str,
        source_type: str,
        trace_id: str,
        media_type: str,
        file_bytes: bytes,
    ) -> OCRResponse:
        logger.info(
            format_log(
                "ocr.started",
                trace_id=trace_id,
                question_id=question_id,
                filename=filename,
                source_type=source_type,
                media_type=media_type,
                backend=self.backend_name,
                size_bytes=len(file_bytes),
            )
        )

        started_at = time.perf_counter()

        if self.backend_name == "mock":
            await asyncio.sleep(settings.ocr_mock_delay_seconds)

        backend_result = await self._backend.extract_text(
            file_bytes=file_bytes,
            filename=filename,
            media_type=media_type,
        )
        structured = self._structurer.structure(
            source_type=source_type,
            filename=filename,
            ocr_result=backend_result,
        )
        structured = await self._refiner.refine(
            source_type=source_type,
            filename=filename,
            result=structured,
        )

        sq = structured.structured_question

        ocr_ms = int((time.perf_counter() - started_at) * 1000)
        response = OCRResponse(
            traceId=trace_id,
            questionId=question_id,
            status="completed",
            rawText=structured.raw_text,
            structuredQuestion=structured.structured_question,
            warnings=structured.warnings,
            cost={
                "ocrMs": ocr_ms,
                "parserMs": structured.parser_ms,
            },
        )
        logger.info(
            format_log(
                "ocr.completed",
                trace_id=trace_id,
                question_id=question_id,
                backend=self.backend_name,
                ocr_ms=ocr_ms,
                parser_ms=structured.parser_ms,
                warnings=structured.warnings,
                stem_len=len(structured.structured_question.stem),
                has_diagram=sq.metadata.hasDiagram,
                diagram_desc_len=len(sq.diagramDescription) if sq.diagramDescription else 0,
            )
        )
        logger.info(
            format_log(
                "ocr.result",
                trace_id=trace_id,
                question_id=question_id,
                question_type=structured.structured_question.questionType,
                options_count=len(structured.structured_question.options),
                diagram_description=structured.structured_question.diagramDescription,
            )
        )
        return response
