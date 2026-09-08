from __future__ import annotations

from typing import Any, Literal

from pydantic import BaseModel, Field


QuestionType = Literal[
    "single_choice",
    "multiple_choice",
    "true_false",
    "fill_blank",
    "subjective",
    "short_answer",
    "essay",
    "calculation",
    "unknown",
]


class OptionItem(BaseModel):
    key: str
    content: str


class QuestionAsset(BaseModel):
    type: str
    role: str = "question_attachment"
    ocrRelated: bool = True
    description: str | None = None
    sourceName: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


class QuestionMetadata(BaseModel):
    sourceType: str
    hasDiagram: bool = False
    ocrConfidence: float | None = None
    importMode: str = "single_question"
    extractionMethod: str | None = None


class StructuredQuestion(BaseModel):
    stem: str
    questionType: QuestionType
    options: list[OptionItem] = Field(default_factory=list)
    assets: list[QuestionAsset] = Field(default_factory=list)
    suggestedAnswer: str | None = None
    rawText: str = ""
    hasDiagram: bool = False
    diagramDescription: str | None = None
    warnings: list[str] = Field(default_factory=list)
    metadata: QuestionMetadata
