"""Offline contract evaluation; it deliberately never calls a real provider."""
from __future__ import annotations

import asyncio
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from app.agents.dispatcher import AgentDispatcher
from app.schemas.agent import AgentActionRequest, QuestionContext
from app.schemas.question import QuestionMetadata, StructuredQuestion


async def main() -> None:
    cases = json.loads((Path(__file__).parent / "p0_cases.json").read_text(encoding="utf-8"))
    question = StructuredQuestion(stem="样例题", questionType="single_choice", options=[], metadata=QuestionMetadata(sourceType="manual"))
    context = QuestionContext(question=question, latestAnswer="A", referenceAnswer="B")
    dispatcher = AgentDispatcher(client=None)
    results = []
    for action, params in [("diagnose_mistake", {}), ("explain_alternative", {"focus": "步骤"}), ("hint", {"hintLevel": 1}), ("suggest_taxonomy", {})]:
        response = await dispatcher.run(AgentActionRequest(traceId=f"eval-{action}", questionId=1, action=action, context=context, params=params))
        results.append({"action": action, "status": response.status, "source": response.meta["source"]})
    print(json.dumps({"caseCount": len(cases), "contractChecks": results, "realProvider": False, "qualityClaims": "none"}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    asyncio.run(main())
