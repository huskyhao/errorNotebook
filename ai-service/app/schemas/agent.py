from __future__ import annotations

from typing import Any, Literal

from pydantic import BaseModel, Field, model_validator

from app.schemas.analysis import AnalysisPayload, TaxonomySuggestion
from app.schemas.question import OptionItem, StructuredQuestion

AgentAction = Literal[
    "diagnose_mistake",
    "explain_alternative",
    "hint",
    "suggest_taxonomy",
    "generate_similar_question",
    "grade_subjective_answer",
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
    # The dispatcher validates the action-specific shape before invoking a
    # provider. Keeping the wire field as a JSON object preserves the P0
    # client contract while the concrete models below make each action typed.
    params: dict[str, Any] = Field(default_factory=dict, max_length=32)


class SimilarQuestionParams(BaseModel):
    targetDifficulty: str | None = Field(default=None, max_length=32)
    allowedKnowledgePoints: list[str] = Field(default_factory=list, max_length=8)
    questionType: str | None = Field(default=None, max_length=32)
    sourceFingerprint: str | None = Field(default=None, max_length=128)


class GradeSubjectiveParams(BaseModel):
    maxScore: float = Field(gt=0, le=100)
    rubric: list[str] = Field(default_factory=list, max_length=32)
    standardAnswer: str | None = Field(default=None, max_length=12000)
    referenceAnalysis: str | None = Field(default=None, max_length=12000)
    scoringPoints: list[str] = Field(default_factory=list, max_length=32)


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


class SimilarQuestionResult(BaseModel):
    proposalId: str = Field(min_length=1, max_length=128)
    sourceQuestionId: int
    sourceFingerprint: str = Field(min_length=1, max_length=128)
    stem: str = Field(min_length=1, max_length=12000)
    questionType: Literal[
        "single_choice", "multiple_choice", "true_false", "fill_blank",
        "subjective", "short_answer", "essay", "calculation",
    ]
    options: list[OptionItem] = Field(default_factory=list, max_length=12)
    answer: str = ""
    analysis: str = Field(min_length=1, max_length=12000)
    knowledgePoints: list[str] = Field(default_factory=list, max_length=12)
    variationStrategy: str = Field(min_length=1, max_length=1000)
    warnings: list[str] = Field(default_factory=list, max_length=12)
    qualityStatus: Literal["ok", "needs_review"] = "ok"


class GradingCriterionResult(BaseModel):
    name: str = Field(min_length=1, max_length=200)
    maxScore: float = Field(ge=0, le=100)
    score: float = Field(ge=0, le=100)
    evidence: list[str] = Field(default_factory=list, max_length=8)

    @model_validator(mode="after")
    def score_within_criterion(self) -> "GradingCriterionResult":
        if self.score > self.maxScore:
            raise ValueError("criterion score exceeds maxScore")
        return self


class GradeSubjectiveResult(BaseModel):
    suggestedScore: float = Field(ge=0, le=100)
    maxScore: float = Field(gt=0, le=100)
    criteriaResults: list[GradingCriterionResult] = Field(default_factory=list, max_length=32)
    strengths: list[str] = Field(default_factory=list, max_length=12)
    missingPoints: list[str] = Field(default_factory=list, max_length=12)
    feedback: str = Field(min_length=1, max_length=12000)
    evidence: list[str] = Field(default_factory=list, max_length=16)
    uncertainties: list[str] = Field(default_factory=list, max_length=12)
    confidence: float = Field(ge=0, le=1)
    requiresHumanReview: bool = True

    @model_validator(mode="after")
    def score_contract(self) -> "GradeSubjectiveResult":
        if self.suggestedScore > self.maxScore:
            raise ValueError("suggestedScore exceeds maxScore")
        if self.criteriaResults:
            total = round(sum(item.score for item in self.criteriaResults), 6)
            if round(total, 6) != round(self.suggestedScore, 6):
                raise ValueError("criteria scores do not sum to suggestedScore")
            if round(sum(item.maxScore for item in self.criteriaResults), 6) != round(self.maxScore, 6):
                raise ValueError("criterion max scores do not sum to maxScore")
        return self


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
        | SimilarQuestionResult
        | GradeSubjectiveResult
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
