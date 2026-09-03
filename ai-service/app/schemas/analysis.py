from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field

from app.schemas.question import StructuredQuestion


class AnalysisRequest(BaseModel):
    questionId: int
    traceId: str
    question: StructuredQuestion
    userAnswer: str | None = None
    context: dict[str, Any] = Field(default_factory=dict)


class AnalysisPayload(BaseModel):
    answer: str
    summary: str
    knowledgePoints: list[str] = Field(default_factory=list)
    steps: list[str] = Field(default_factory=list)
    optionAnalysis: dict[str, str] = Field(default_factory=dict)
    pitfalls: list[str] = Field(default_factory=list)
    reviewAdvice: list[str] = Field(default_factory=list)


class AnalysisResponse(BaseModel):
    traceId: str
    questionId: int
    status: str
    analysis: AnalysisPayload
    cost: dict[str, int]
