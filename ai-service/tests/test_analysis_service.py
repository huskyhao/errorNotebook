import asyncio
import json
import unittest
from types import SimpleNamespace
from unittest.mock import AsyncMock, patch

from app.schemas.analysis import AnalysisPayload, AnalysisRequest, TaxonomySuggestion
from app.schemas.question import QuestionMetadata, StructuredQuestion
from app.services.analysis_service import AnalysisService
from app.services.openai_client import OpenAICompatibleError


def make_request() -> AnalysisRequest:
    return AnalysisRequest(
        questionId=1,
        traceId="trace-test",
        question=StructuredQuestion(
            stem="下列说法正确的是？",
            questionType="single_choice",
            options=[
                {"key": "A", "content": "选项 A"},
                {"key": "B", "content": "选项 B"},
            ],
            suggestedAnswer="A",
            rawText="下列说法正确的是？ A. 选项 A B. 选项 B",
            metadata=QuestionMetadata(sourceType="image"),
        ),
        context={"categoryCandidates": [], "tagCandidates": []},
    )


def make_subjective_request() -> AnalysisRequest:
    return AnalysisRequest(
        questionId=2,
        traceId="trace-subjective",
        question=StructuredQuestion(
            stem="说明查找与 K 差值最小结点的算法思想，并用 C/C++ 描述算法。",
            questionType="subjective",
            options=[],
            rawText="说明算法思想，并用 C/C++ 描述算法。",
            metadata=QuestionMetadata(sourceType="image"),
        ),
        context={"categoryCandidates": [], "tagCandidates": []},
    )


class SequencedClient:
    def __init__(self, responses: list[str]) -> None:
        self.responses = responses
        self.user_prompts: list[str] = []

    async def create_structured_completion(self, *, system_prompt: str, user_prompt: str) -> tuple[str, int]:
        self.user_prompts.append(user_prompt)
        return self.responses.pop(0), 20


class AnalysisServiceTest(unittest.IsolatedAsyncioTestCase):
    async def test_taxonomy_keeps_broad_candidate_and_allows_new_knowledge_tags(self) -> None:
        request = make_request().model_copy(
            update={
                "context": {
                    "categoryCandidates": ["数据结构", "操作系统"],
                    "tagCandidates": ["树"],
                }
            }
        )
        analysis = AnalysisPayload(
            answer="A",
            summary="解析",
            taxonomySuggestion=TaxonomySuggestion(
                categoryName="数据结构",
                tagNames=["树", "图的定义", " 图的定义 ", "x", "含\n换行"],
                confidence=1.2,
            ),
        )

        result, warnings = AnalysisService._validate_domain(analysis, request)

        self.assertEqual(result.taxonomySuggestion.categoryName, "数据结构")
        self.assertEqual(result.taxonomySuggestion.tagNames, ["树", "图的定义"])
        self.assertEqual(result.taxonomySuggestion.confidence, 1)
        self.assertNotIn("taxonomy_category_outside_candidates", warnings)

    async def test_taxonomy_rejects_fine_grained_category_outside_candidates(self) -> None:
        request = make_request().model_copy(
            update={"context": {"categoryCandidates": ["数据结构"], "tagCandidates": []}}
        )
        analysis = AnalysisPayload(
            answer="A",
            summary="解析",
            taxonomySuggestion=TaxonomySuggestion(
                categoryName="图论", tagNames=["生成树"]
            ),
        )

        result, warnings = AnalysisService._validate_domain(analysis, request)

        self.assertIsNone(result.taxonomySuggestion.categoryName)
        self.assertEqual(result.taxonomySuggestion.tagNames, ["生成树"])
        self.assertIn("taxonomy_category_outside_candidates", warnings)

    async def test_image_analysis_falls_back_to_ocr_text_provider(self) -> None:
        service = AnalysisService.__new__(AnalysisService)
        service._client = object()
        service._multimodal_client = SimpleNamespace(model="fake-vision")
        service._analyze_with_multimodal = AsyncMock(
            side_effect=OpenAICompatibleError(
                "PROVIDER_TIMEOUT", "vision unavailable", True, 504
            )
        )
        service._analyze_with_openai = AsyncMock(
            return_value=(
                AnalysisPayload(
                    answer="A",
                    summary="基于 OCR 题面完成的解析",
                    optionAnalysis={"A": "正确", "B": "错误"},
                ),
                12,
            )
        )

        response = await service.analyze_question(
            make_request(), image_bytes=b"PNG", media_type="image/png"
        )

        service._analyze_with_multimodal.assert_awaited_once()
        service._analyze_with_openai.assert_awaited_once()
        self.assertEqual(response.analysis.summary, "基于 OCR 题面完成的解析")
        self.assertIn("multimodal_fallback_to_ocr", response.warnings)
        self.assertEqual(response.status, "needs_review")

    async def test_image_timeout_without_text_provider_is_explicit_error(self) -> None:
        service = AnalysisService.__new__(AnalysisService)
        service._client = None
        service._multimodal_client = SimpleNamespace(model="fake-vision")
        service._analyze_with_multimodal = AsyncMock(side_effect=asyncio.TimeoutError())

        with self.assertRaises(OpenAICompatibleError) as raised:
            await service.analyze_question(
                make_request(), image_bytes=b"PNG", media_type="image/png"
            )

        self.assertEqual(raised.exception.code, "PROVIDER_TIMEOUT")

    async def test_multiple_choice_answer_is_validated_as_a_set_of_option_keys(self) -> None:
        request = make_request().model_copy(update={
            "question": make_request().question.model_copy(update={"questionType": "multiple_choice"}),
        })
        analysis = AnalysisPayload(answer="B, A", summary="解析")

        result, warnings = AnalysisService._validate_domain(analysis, request)

        self.assertEqual(result.answer, "A,B")
        self.assertNotIn("answer_not_in_options", warnings)

    async def test_true_false_answer_uses_boolean_contract(self) -> None:
        request = make_request().model_copy(update={
            "question": make_request().question.model_copy(update={"questionType": "true_false", "options": []}),
        })
        result, warnings = AnalysisService._validate_domain(AnalysisPayload(answer="正确", summary="解析"), request)

        self.assertEqual(result.answer, "true")
        self.assertNotIn("answer_not_boolean", warnings)

    async def test_subjective_placeholder_is_rejected_and_not_saved_as_answer(self) -> None:
        analysis = AnalysisPayload(
            answer="参考答案见解析",
            summary="这里只给出了解析摘要",
        )

        with self.assertRaisesRegex(ValueError, "subjective_answer_placeholder"):
            AnalysisService._parse_complete_analysis(
                json.dumps(analysis.model_dump(), ensure_ascii=False), make_subjective_request()
            )

        result, warnings = AnalysisService._validate_domain(analysis, make_subjective_request())
        self.assertEqual(result.answer, "")
        self.assertIn("subjective_answer_placeholder", warnings)
        self.assertIn("answer_unconfirmed", warnings)

    async def test_subjective_code_requirement_needs_code_in_answer(self) -> None:
        prose_only = AnalysisPayload(
            answer="先遍历二叉搜索树，记录当前最小差值和对应结点，最后输出结果。",
            summary="算法说明",
        )
        self.assertEqual(
            AnalysisService._free_text_answer_issue(prose_only, make_subjective_request()),
            "subjective_answer_missing_code",
        )

        complete = prose_only.model_copy(
            update={
                "answer": (
                    "算法思想：利用二叉搜索树有序性递归搜索并维护最优值。\n"
                    "```cpp\nvoid find(Node* p, int K) { if (!p) return; find(p->left, K); "
                    "/* 更新最小差值 */ find(p->right, K); }\n```"
                )
            }
        )
        self.assertIsNone(AnalysisService._free_text_answer_issue(complete, make_subjective_request()))

    async def test_text_provider_repairs_subjective_placeholder_once(self) -> None:
        invalid = AnalysisPayload(answer="参考答案见解析", summary="解析摘要")
        valid = AnalysisPayload(
            answer=(
                "算法思想：中序遍历二叉搜索树并维护最小差值。\n"
                "```cpp\nvoid find(Node* p, int K) { if (!p) return; find(p->left, K); "
                "/* 比较并记录 */ find(p->right, K); }\n```"
            ),
            summary="完整解析",
        )
        client = SequencedClient([
            json.dumps(invalid.model_dump(), ensure_ascii=False),
            json.dumps(valid.model_dump(), ensure_ascii=False),
        ])
        service = AnalysisService.__new__(AnalysisService)
        service._client = client

        fake_settings = SimpleNamespace(ai_max_attempts=2, ai_action_timeout_seconds=2)
        with patch("app.services.analysis_service.settings", fake_settings):
            result, _ = await service._analyze_with_openai(make_subjective_request())

        self.assertEqual(result.answer, valid.answer)
        self.assertEqual(len(client.user_prompts), 2)
        self.assertIn("subjective_answer_placeholder", client.user_prompts[1])
        self.assertIn("必须把代码直接写入 answer", client.user_prompts[1])


if __name__ == "__main__":
    unittest.main()
