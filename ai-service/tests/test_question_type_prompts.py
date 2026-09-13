import unittest

from app.services.question_type_prompts import infer_question_type, question_type_guidance


class QuestionTypePromptTest(unittest.TestCase):
    def test_infers_non_single_types_from_low_risk_markers(self) -> None:
        self.assertEqual(infer_question_type("多选题：下列哪些正确", "", True), "multiple_choice")
        self.assertEqual(infer_question_type("判断题：说法是否正确", "", True), "true_false")
        self.assertEqual(infer_question_type("请在括号中填空", "", False), "fill_blank")
        self.assertEqual(infer_question_type("计算题：求加速度", "", False), "calculation")

    def test_unknown_type_has_conservative_guidance(self) -> None:
        self.assertIn("保守", question_type_guidance("not_supported"))


if __name__ == "__main__":
    unittest.main()
