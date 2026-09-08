from __future__ import annotations

from typing import Any, Literal

from pydantic import BaseModel, Field, model_validator

from app.schemas.analysis import AnalysisPayload, TaxonomySuggestion
from app.schemas.question import StructuredQuestion

AgentAction = Literal[
    "diagnose_mistake",
    "explain_alternative",
    "hint",
    "suggest_taxonomy",
]
ActionStatus = Literal["completed", "needs_input", "needs_review", "failed"]


class ContextMessage(BaseModel):
    role: Literal["user", "assistant"]
    content: str = Field(min_length=1, max_length=4000)


class QuestionContext(BaseModel):
    question: StructuredQuestion
    warnings: list[str] = Field(default_factory=list, max_length=32)
    referenceAnswer: str | None = None
    referenceAnswerSource: str | None = None
    latestAnswer: str | None = None
    analysis: AnalysisPayload | None = None
    conversation: list[ContextMessage] = Field(default_factory=list, max_length=20)
    categoryCandidates: list[str] = Field(default_factory=list, max_length=100)
    tagCandidates: list[str] = Field(default_factory=list, max_length=200)
    contentFingerprint: str | None = Field(default=None, max_length=128)
    version: str | None = Field(default=None, max_length=128)


class AgentActionRequest(BaseModel):
    traceId: str = Field(min_length=1, max_length=128)
    questionId: int
    action: AgentAction
    context: QuestionContext
    params: dict[str, Any] = Field(default_factory=dict)


class DiagnoseMistakeResult(BaseModel):
    mistakeReason: str | None = None
    reasonType: Literal[
        "概念混淆", "条件遗漏", "计算错误", "方法错误", "证据不足"
    ]
    evidence: list[str] = Field(default_factory=list, max_length=8)
    weaknessTags: list[str] = Field(default_factory=list, max_length=8)
    reviewAdvice: list[str] = Field(default_factory=list, max_length=8)
    uncertainties: list[str] = Field(default_factory=list, max_length=8)


class ExplainAlternativeResult(BaseModel):
    explanation: str
    focusPoints: list[str] = Field(default_factory=list, max_length=8)
    checkQuestion: str
    needsReview: bool = False


class HintResult(BaseModel):
    hintLevel: Literal[1, 2, 3]
    hint: str
    nextQuestion: str
    revealsAnswer: bool


class SuggestTaxonomyResult(BaseModel):
    taxonomySuggestion: TaxonomySuggestion
    reason: str


class AgentActionError(BaseModel):
    code: str
    message: str
    retryable: bool = False
    traceId: str | None = None


class AgentActionResponse(BaseModel):
    traceId: str
    questionId: int
    action: AgentAction
    status: ActionStatus
    result: (
        DiagnoseMistakeResult
        | ExplainAlternativeResult
        | HintResult
        | SuggestTaxonomyResult
        | None
    ) = None
    warnings: list[str] = Field(default_factory=list)
    error: AgentActionError | None = None
    meta: dict[str, Any] = Field(default_factory=dict)

    @model_validator(mode="after")
    def validate_result_for_status(self) -> "AgentActionResponse":
        if self.status == "completed" and self.result is None:
            raise ValueError("completed action must include result")
        if self.status == "failed" and self.error is None:
            raise ValueError("failed action must include error")
        return self
