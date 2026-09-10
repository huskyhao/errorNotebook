import asyncio
import unittest
from types import SimpleNamespace
from unittest.mock import AsyncMock

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


if __name__ == "__main__":
    unittest.main()
