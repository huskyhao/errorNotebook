import unittest

from app.agents.dispatcher import AgentDispatcher
from app.schemas.agent import AgentActionRequest, QuestionContext
from app.schemas.chat import ChatImage, ChatRequest
from app.schemas.question import QuestionMetadata, StructuredQuestion
from app.services.chat_service import ChatService


def context(**changes):
    value = QuestionContext(
        question=StructuredQuestion(
            stem="一个物体在斜面上运动，求加速度。", questionType="calculation", options=[],
            suggestedAnswer="", rawText="", warnings=[], metadata=QuestionMetadata(sourceType="manual")
        ), latestAnswer="a=2m/s^2", referenceAnswer="a=2m/s^2", contentFingerprint="sha256:source",
        categoryCandidates=["物理"], tagCandidates=["牛顿定律", "受力分析"],
    )
    for key, value2 in changes.items():
        setattr(value, key, value2)
    return value


class P1ActionsTest(unittest.IsolatedAsyncioTestCase):
    async def test_similar_question_is_structured_and_needs_review_in_mock(self):
        response = await AgentDispatcher().run(AgentActionRequest(
            traceId="similar", questionId=7, action="generate_similar_question",
            context=context(), params={"sourceFingerprint": "sha256:source"},
        ))
        self.assertEqual(response.status, "needs_review")
        self.assertEqual(response.result.sourceQuestionId, 7)
        self.assertEqual(response.result.sourceFingerprint, "sha256:source")
        self.assertTrue(response.result.proposalId)

    async def test_subjective_grade_has_basis_and_bounded_score(self):
        response = await AgentDispatcher().run(AgentActionRequest(
            traceId="grade", questionId=7, action="grade_subjective_answer", context=context(),
            params={"maxScore": 10, "rubric": ["受力分析", "计算过程"]},
        ))
        self.assertEqual(response.status, "needs_review")
        result = response.result
        self.assertGreaterEqual(result.suggestedScore, 0)
        self.assertLessEqual(result.suggestedScore, result.maxScore)
        self.assertEqual(sum(item.score for item in result.criteriaResults), result.suggestedScore)
        self.assertTrue(result.requiresHumanReview)

    async def test_grade_requires_basis_and_objective_is_rejected(self):
        missing = await AgentDispatcher().run(AgentActionRequest(
            traceId="missing", questionId=7, action="grade_subjective_answer", context=context(), params={"maxScore": 10},
        ))
        self.assertEqual(missing.status, "needs_input")
        objective = context()
        objective.question.questionType = "single_choice"
        rejected = await AgentDispatcher().run(AgentActionRequest(
            traceId="objective", questionId=7, action="grade_subjective_answer", context=objective,
            params={"maxScore": 10, "standardAnswer": "A"},
        ))
        self.assertEqual(rejected.status, "needs_input")

    async def test_visual_chat_receives_actual_bytes_in_order(self):
        class FakeVision:
            def __init__(self): self.images = None
            async def create_chat_completion_with_images(self, *, images, system_prompt, user_prompt):
                self.images = images
                return "我看到了图中的受力方向。", 3

        vision = FakeVision()
        service = ChatService(client=None, vision_client=vision)
        request = ChatRequest(
            questionId=1, traceId="image-chat", question=context().question,
            message="图中这一步为什么这样列式？",
            imageAttachments=[
                ChatImage(fileName="a.png", contentType="image/png", content=b"PNG-A"),
                ChatImage(fileName="b.png", contentType="image/png", content=b"PNG-B"),
            ],
        )
        response = await service.chat(request)
        self.assertEqual(response.status, "completed")
        self.assertEqual([item[0] for item in vision.images], [b"PNG-A", b"PNG-B"])


if __name__ == "__main__":
    unittest.main()
