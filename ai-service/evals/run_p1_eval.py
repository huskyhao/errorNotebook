"""Offline P1 contract/quality gates. It intentionally makes no quality claim about a real model."""
from __future__ import annotations

import json
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from app.agents.dispatcher import AgentDispatcher
from app.schemas.agent import AgentActionRequest, QuestionContext
from app.schemas.question import QuestionMetadata, StructuredQuestion


def main() -> None:
    cases = json.loads((Path(__file__).with_name("p1_cases.json")).read_text(encoding="utf-8"))
    assert all(case.get("expected") and case.get("forbidden") for case in cases)
    families = {family: [case for case in cases if case["family"] == family] for family in {case["family"] for case in cases}}
    assert all(len(families.get(family, [])) >= 6 for family in ("taxonomy", "image", "similar", "grading"))
    context = QuestionContext(
        question=StructuredQuestion(stem="求函数最小值", questionType="calculation", options=[], suggestedAnswer="2", rawText="求函数最小值", warnings=[], metadata=QuestionMetadata(sourceType="manual")),
        latestAnswer="1", referenceAnswer="2", contentFingerprint="sha256:eval", categoryCandidates=["数学"], tagCandidates=["函数"],
    )
    import asyncio
    async def run():
        taxonomy = await AgentDispatcher().run(AgentActionRequest(traceId="eval-tax", questionId=1, action="suggest_taxonomy", context=context))
        similar = await AgentDispatcher().run(AgentActionRequest(traceId="eval-sim", questionId=1, action="generate_similar_question", context=context, params={"sourceFingerprint":"sha256:eval"}))
        grade = await AgentDispatcher().run(AgentActionRequest(traceId="eval-grade", questionId=1, action="grade_subjective_answer", context=context, params={"maxScore":10, "scoringPoints":["过程"]}))
        assert taxonomy.result.taxonomySuggestion.categoryName in {"数学", None}
        assert similar.result.qualityStatus in {"ok", "needs_review"} and similar.result.proposalId
        assert 0 <= grade.result.suggestedScore <= grade.result.maxScore
        return taxonomy, similar, grade
    taxonomy, similar, grade = asyncio.run(run())
    print(json.dumps({"caseCount": len(cases), "families": {key: len(value) for key, value in families.items()}, "mockOnly": True, "qualityClaims": "none", "checks": {"taxonomy": taxonomy.status, "similar": similar.status, "grading": grade.status}}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
