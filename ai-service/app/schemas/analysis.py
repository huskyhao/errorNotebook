from __future__ import annotations

from typing import Any, Literal

from pydantic import BaseModel, Field

from app.schemas.question import StructuredQuestion


class AnalysisRequest(BaseModel):
    questionId: int
    traceId: str
    question: StructuredQuestion
    userAnswer: str | None = None
    context: dict[str, Any] = Field(default_factory=dict)


class TaxonomySuggestion(BaseModel):
    """Advisory taxonomy output; the Go service must validate before apply."""

    categoryName: str | None = None
    tagNames: list[str] = Field(default_factory=list)
    confidence: float | None = None


class TaxonomySuggestionReason(BaseModel):
    reason: str = ""


class AnalysisPayload(BaseModel):
    answer: str
    answerFormat: str = "free_text"
    blankAnswers: list[str] = Field(default_factory=list)
    scoringPoints: list[str] = Field(default_factory=list)
    rubric: list[str] = Field(default_factory=list)
    summary: str
    knowledgePoints: list[str] = Field(default_factory=list)
    taxonomySuggestion: TaxonomySuggestion | None = None
    taxonomySuggestionReason: str | None = None
    steps: list[str] = Field(default_factory=list)
    optionAnalysis: dict[str, str] = Field(default_factory=dict)
    pitfalls: list[str] = Field(default_factory=list)
    reviewAdvice: list[str] = Field(default_factory=list)


class AnalysisResponse(BaseModel):
    traceId: str
    questionId: int
    status: Literal["completed", "needs_review", "failed"]
    analysis: AnalysisPayload
    cost: dict[str, int | None] = Field(default_factory=dict)
    warnings: list[str] = Field(default_factory=list)
    error: dict[str, object] | None = None
