import unittest

from app.services.ocr_backends import OCRBackendResult, OCRTextBlock
from app.services.question_structure_refiner import QuestionStructureRefiner
from app.services.question_structurer import QuestionStructurer


class QuestionStructurerTest(unittest.TestCase):
    def test_chinese_period_option_separator_is_detected(self) -> None:
        ocr = OCRBackendResult(
            blocks=[
                OCRTextBlock(text="以下说法正确的是", confidence=0.95),
                OCRTextBlock(text="A.选项一", confidence=0.95),
                OCRTextBlock(text="D。以上都不正确", confidence=0.95),
            ],
            engine="mock",
        )

        result = QuestionStructurer().structure(source_type="image", filename="q.png", ocr_result=ocr)

        self.assertEqual(result.structured_question.stem, "以下说法正确的是")
        self.assertIn(("D", "以上都不正确"), [(item.key, item.content) for item in result.structured_question.options])

    def test_removes_option_noise_and_keeps_stem_clean(self) -> None:
        ocr = OCRBackendResult(
            blocks=[
                OCRTextBlock(text="这道题的正确结果是", confidence=0.93),
                OCRTextBlock(text="OA.", confidence=0.91),
                OCRTextBlock(text="4", confidence=0.92),
                OCRTextBlock(text="O", confidence=0.88),
                OCRTextBlock(text="B.5", confidence=0.94),
                OCRTextBlock(text="O", confidence=0.88),
                OCRTextBlock(text=".", confidence=0.82),
                OCRTextBlock(text="D.7", confidence=0.94),
            ],
            engine="mock",
        )

        result = QuestionStructurer().structure(source_type="image", filename="q.png", ocr_result=ocr)

        self.assertEqual(result.structured_question.stem, "这道题的正确结果是")
        self.assertEqual(
            [(item.key, item.content) for item in result.structured_question.options],
            [("A", "4"), ("B", "5"), ("D", "7")],
        )
        self.assertIn("options_incomplete", result.warnings)
        self.assertIn("missing_options_C", result.warnings)
        self.assertNotIn("OA", result.structured_question.stem)

    def test_single_choice_options_are_recovered_from_circle_prefix(self) -> None:
        ocr = OCRBackendResult(
            blocks=[
                OCRTextBlock(text="下列说法正确的是", confidence=0.96),
                OCRTextBlock(text="O A.速度是矢量", confidence=0.95),
                OCRTextBlock(text="O B.路程是矢量", confidence=0.95),
                OCRTextBlock(text="O C.时间是矢量", confidence=0.95),
                OCRTextBlock(text="O D.质量是矢量", confidence=0.95),
            ],
            engine="mock",
        )

        result = QuestionStructurer().structure(source_type="image", filename="q.png", ocr_result=ocr)

        self.assertEqual(result.structured_question.questionType, "single_choice")
        self.assertEqual(len(result.structured_question.options), 4)
        self.assertEqual(result.structured_question.options[0].key, "A")

    def test_exam_ui_prefix_is_removed_from_stem(self) -> None:
        ocr = OCRBackendResult(
            blocks=[
                OCRTextBlock(
                    text="倒计时00:17:35 24/31单选题（分值3.0分，难度：易） 下列哪一种图不一定是树（）。",
                    confidence=0.96,
                ),
                OCRTextBlock(text="A.完全图", confidence=0.95),
                OCRTextBlock(text="B.最小生成树", confidence=0.95),
                OCRTextBlock(text="C.二叉树", confidence=0.95),
                OCRTextBlock(text="D.线索二叉树", confidence=0.95),
            ],
            engine="mock",
        )

        result = QuestionStructurer().structure(
            source_type="image", filename="graph.png", ocr_result=ocr
        )

        self.assertEqual(result.structured_question.stem, "下列哪一种图不一定是树（）。")
        self.assertIn("ocr_exam_ui_noise_removed", result.warnings)

    def test_exam_ui_lines_are_dropped_before_stem(self) -> None:
        ocr = OCRBackendResult(
            blocks=[
                OCRTextBlock(text="倒计时 00:17:35", confidence=0.96),
                OCRTextBlock(text="24/31", confidence=0.96),
                OCRTextBlock(text="单选题（分值3.0分，难度：易）", confidence=0.96),
                OCRTextBlock(text="下列哪一种图不一定是树（）。", confidence=0.96),
            ],
            engine="mock",
        )

        result = QuestionStructurer().structure(
            source_type="image", filename="graph.png", ocr_result=ocr
        )

        self.assertEqual(result.structured_question.stem, "下列哪一种图不一定是树（）。")


class _MockRefineClient:
    def __init__(self, content: str) -> None:
        self.content = content

    async def create_structured_completion(self, *, system_prompt: str, user_prompt: str) -> tuple[str, int]:
        return self.content, 0


class _FailingRefineClient:
    async def create_structured_completion(self, *, system_prompt: str, user_prompt: str) -> tuple[str, int]:
        raise RuntimeError("boom")


class QuestionStructureRefinerTest(unittest.IsolatedAsyncioTestCase):
    async def test_llm_refine_recovers_philosopher_options_and_clean_stem(self) -> None:
        raw_lines = [
            '解决"就餐哲学家问题"并避免死锁的一种方法是（）：',
            "OA.确保所有哲学家先拿左边的叉子再拿右边的叉子",
            "B.确保所有哲学家先拿右边的叉子再拿左边的叉子",
            "确保某一位特定哲学家先拿左边的叉子再拿右边的叉子，而其他所有哲学家先拿右边的",
            "叉子再拿左边的叉子",
            "D。以上都不正确",
        ]
        ocr = OCRBackendResult(blocks=[OCRTextBlock(text=line, confidence=0.93) for line in raw_lines], engine="mock")
        rule_result = QuestionStructurer().structure(source_type="image", filename="philosopher.png", ocr_result=ocr)
        client = _MockRefineClient(
            """
            {
              "stem": "解决\\"就餐哲学家问题\\"并避免死锁的一种方法是（）：",
              "questionType": "single_choice",
              "options": [
                {"key": "A", "content": "确保所有哲学家先拿左边的叉子再拿右边的叉子"},
                {"key": "B", "content": "确保所有哲学家先拿右边的叉子再拿左边的叉子"},
                {"key": "C", "content": "确保某一位特定哲学家先拿左边的叉子再拿右边的叉子，而其他所有哲学家先拿右边的叉子再拿左边的叉子"},
                {"key": "D", "content": "以上都不正确"}
              ],
              "warnings": ["ocr_option_noise_removed"]
            }
            """
        )

        refined = await QuestionStructureRefiner(client=client).refine(source_type="image", filename="philosopher.png", result=rule_result)

        self.assertEqual(refined.structured_question.stem, '解决"就餐哲学家问题"并避免死锁的一种方法是（）：')
        self.assertEqual([item.key for item in refined.structured_question.options], ["A", "B", "C", "D"])
        self.assertIn("llm_refined", refined.warnings)
        self.assertNotIn("以上都不正确", refined.structured_question.stem)

    async def test_llm_refine_failure_falls_back_with_warning(self) -> None:
        ocr = OCRBackendResult(
            blocks=[
                OCRTextBlock(text="题干", confidence=0.91),
                OCRTextBlock(text="OA.", confidence=0.91),
                OCRTextBlock(text="1", confidence=0.91),
            ],
            engine="mock",
        )
        rule_result = QuestionStructurer().structure(source_type="image", filename="q.png", ocr_result=ocr)

        refined = await QuestionStructureRefiner(client=_FailingRefineClient()).refine(source_type="image", filename="q.png", result=rule_result)

        self.assertEqual(refined.structured_question.stem, rule_result.structured_question.stem)
        self.assertIn("llm_refine_failed", refined.warnings)

    async def test_lru_missing_c_keeps_abd_and_lowers_confidence(self) -> None:
        raw_lines = [
            "考虑一个全相联缓存，共有8个缓存块（编号0-7），并有以下内存块请求序列：",
            "4,3,25,8,19,6,25,8,16, 35, 45,22,8,3,16,25,7",
            "如果使用LRU替换策略，内存块7最终会存储在哪个缓存块中？",
            "OA.",
            "4",
            "O",
            "B.5",
            "O",
            ".",
            "D.7",
        ]
        ocr = OCRBackendResult(blocks=[OCRTextBlock(text=line, confidence=0.94) for line in raw_lines], engine="mock")
        rule_result = QuestionStructurer().structure(source_type="image", filename="lru.png", ocr_result=ocr)

        refined = await QuestionStructureRefiner(use_default_client=False).refine(source_type="image", filename="lru.png", result=rule_result)

        self.assertEqual([(item.key, item.content) for item in refined.structured_question.options], [("A", "4"), ("B", "5"), ("D", "7")])
        self.assertIn("missing_options_C", refined.warnings)
        self.assertLessEqual(refined.structured_question.metadata.ocrConfidence or 1, 0.62)


if __name__ == "__main__":
    unittest.main()
