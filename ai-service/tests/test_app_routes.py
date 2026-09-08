import unittest

from app.main import app


class AppRoutesTest(unittest.TestCase):
    def test_core_ai_routes_remain_and_exam_parser_route_is_removed(self) -> None:
        paths = set(app.openapi()["paths"])

        self.assertIn("/internal/v1/ocr/parse", paths)
        self.assertIn("/internal/v1/analyze/question", paths)
        self.assertIn("/internal/v1/chat/question", paths)
        self.assertIn("/internal/v1/agent/actions", paths)
        self.assertNotIn("/api/exam/parse", paths)


if __name__ == "__main__":
    unittest.main()
