from __future__ import annotations

from pydantic import BaseModel, Field

from app.schemas.analysis import AnalysisPayload
from app.schemas.question import StructuredQuestion


class ChatMessageItem(BaseModel):
    role: str
    content: str


class ChatRequest(BaseModel):
    questionId: int
    traceId: str
    question: StructuredQuestion
    userAnswer: str | None = None
    analysis: AnalysisPayload | None = None
    history: list[ChatMessageItem] = Field(default_factory=list)
    message: str


class ChatResponse(BaseModel):
    traceId: str
    questionId: int
    status: str
    reply: str
    cost: dict[str, int] = Field(default_factory=dict)
