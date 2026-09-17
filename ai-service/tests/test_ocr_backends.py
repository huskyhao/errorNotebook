import unittest
from types import SimpleNamespace
from unittest.mock import AsyncMock, patch

from app.services.ai_errors import AIServiceError
from app.services.ocr_backends import (
    UnavailableOCRBackend,
    VisionOCRBackend,
    build_ocr_backend,
)


class OCRBackendsTest(unittest.IsolatedAsyncioTestCase):
    async def test_vision_backend_returns_transcribed_lines(self) -> None:
        client = SimpleNamespace(
            create_structured_completion_with_image=AsyncMock(
                return_value=("题干\nA. 选项一\nB. 选项二", 20)
            )
        )
        backend = VisionOCRBackend(client)

        result = await backend.extract_text(
            file_bytes=b"image",
            filename="question.png",
            media_type="image/png",
        )

        self.assertEqual(result.engine, "vision")
        self.assertEqual([block.text for block in result.blocks], ["题干", "A. 选项一", "B. 选项二"])

    async def test_unavailable_backend_fails_instead_of_returning_mock_question(self) -> None:
        backend = UnavailableOCRBackend()

        with self.assertRaises(AIServiceError) as raised:
            await backend.extract_text(
                file_bytes=b"image",
                filename="question.png",
                media_type="image/png",
            )

        self.assertEqual(raised.exception.code, "OCR_PROVIDER_UNAVAILABLE")

    def test_auto_prefers_configured_vision_backend(self) -> None:
        fake_client = SimpleNamespace()
        with (
            patch("app.services.ocr_backends.settings", SimpleNamespace(ocr_backend="auto")),
            patch("app.services.ocr_backends.build_multimodal_client", return_value=fake_client),
        ):
            backend = build_ocr_backend()

        self.assertIsInstance(backend, VisionOCRBackend)


if __name__ == "__main__":
    unittest.main()
