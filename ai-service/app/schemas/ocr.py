from __future__ import annotations

from pydantic import BaseModel, Field

from app.schemas.question import StructuredQuestion


class OCRResponse(BaseModel):
    traceId: str
    questionId: int
    status: str
    rawText: str
    structuredQuestion: StructuredQuestion
    warnings: list[str] = Field(default_factory=list)
    cost: dict[str, int]
