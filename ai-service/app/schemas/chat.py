from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field

from app.schemas.analysis import AnalysisPayload
from app.schemas.question import StructuredQuestion


class ChatMessageItem(BaseModel):
    role: Literal["user", "assistant"]
    content: str = Field(min_length=1, max_length=4000)


class ChatRequest(BaseModel):
    questionId: int
    traceId: str
    question: StructuredQuestion
    userAnswer: str | None = None
    analysis: AnalysisPayload | None = None
    history: list[ChatMessageItem] = Field(default_factory=list, max_length=20)
    message: str = Field(min_length=1, max_length=4000)
    # Populated only by the multipart adapter; never accepted as a client
    # JSON field. Actual bytes are required for visual follow-up.
    imageAttachments: list["ChatImage"] = Field(default_factory=list, exclude=True, max_length=4)


class ChatImage(BaseModel):
    fileName: str = Field(min_length=1, max_length=255)
    contentType: Literal["image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp"]
    content: bytes = Field(min_length=1, max_length=8 * 1024 * 1024)


class ChatResponse(BaseModel):
    traceId: str
    questionId: int
    status: str
    reply: str
    cost: dict[str, int | None] = Field(default_factory=dict)
    error: dict[str, object] | None = None
