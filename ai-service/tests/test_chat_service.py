import unittest

from app.schemas.chat import ChatMessageItem, ChatRequest
from app.schemas.question import QuestionMetadata, StructuredQuestion
from app.services.chat_service import ChatService


class ChatServiceTest(unittest.IsolatedAsyncioTestCase):
    async def test_mock_chat_returns_question_grounded_reply(self) -> None:
        service = ChatService()
        service._client = None
        request = ChatRequest(
            questionId=72,
            traceId="chat-test",
            question=StructuredQuestion(
                stem="下列说法正确的是？",
                questionType="single_choice",
                suggestedAnswer="C",
                metadata=QuestionMetadata(sourceType="image"),
            ),
            history=[ChatMessageItem(role="user", content="上一轮问题")],
            message="为什么选 C？",
        )

        response = await service.chat(request)

        self.assertEqual(response.status, "completed")
        self.assertIn("第 2 轮追问", response.reply)
        self.assertIn("参考答案为 C", response.reply)
        self.assertIn("为什么选 C", response.reply)
        self.assertNotIn("占位回复", response.reply)


if __name__ == "__main__":
    unittest.main()
