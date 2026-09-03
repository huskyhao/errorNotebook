from __future__ import annotations

import asyncio
import logging
import time

from app.core.logging import format_log
from app.schemas.chat import ChatMessageItem, ChatRequest, ChatResponse
from app.services.openai_client import OpenAICompatibleError, build_openai_client

logger = logging.getLogger("app.services.chat_service")

SYSTEM_PROMPT = """你是 ErroNotebook 的 AI 答疑助手，正在辅导学生解答一道题目。

## 你的角色
你是一位经验丰富的老师，耐心、清晰、善于用通俗的语言解释复杂的概念。

## 题目信息
- 题干：{stem}
- 题型：{questionType}
{options_section}
## 用户的作答
{user_answer}

## AI 解析参考
{analysis_section}
## 回答要求
1. 紧扣题目内容，引用题干和选项时保持准确。
2. 参考已有的 AI 解析，可以用更通俗的语言解释其中的概念。
3. 如果用户的问题超出题目范围，合理引导回本题。
4. 用中文回答，语气像一位有经验的老师。
5. 直接输出自然语言，不要输出 JSON 或 markdown 代码块。
6. 回答应简洁有条理，但必要时可以展开详细讲解。
7. 如果用户问的是某个选项为什么对/错，请先给出结论再逐步分析。
"""

class ChatService:
    def __init__(self) -> None:
        self._client = build_openai_client()

    async def chat(self, request: ChatRequest) -> ChatResponse:
        system_prompt = self._build_system_prompt(request)

        messages: list[dict] = [{"role": "system", "content": system_prompt}]
        for msg in request.history:
            messages.append({"role": msg.role, "content": msg.content})
        messages.append({"role": "user", "content": request.message})

        start_ms = int(time.time() * 1000)
        if self._client is None:
            await asyncio.sleep(1)
            reply = self._build_mock_reply(request)
            cost = {}
        else:
            try:
                reply, tokens = await self._client.create_chat_completion(messages=messages)
                cost = {"completionTokens": tokens}
            except OpenAICompatibleError:
                logger.exception(format_log("chat.llm_failed", question_id=request.questionId))
                reply = "抱歉，AI 服务暂时不可用，请稍后重试。"
                cost = {}

        elapsed_ms = int(time.time() * 1000) - start_ms
        logger.info(
            format_log(
                "chat.completed",
                question_id=request.questionId,
                trace_id=request.traceId,
                history_len=len(request.history),
                reply_len=len(reply),
                chat_ms=elapsed_ms,
                **cost,
            )
        )

        return ChatResponse(
            traceId=request.traceId,
            questionId=request.questionId,
            status="completed",
            reply=reply,
            cost=cost,
        )

    def _build_mock_reply(self, request: ChatRequest) -> str:
        question = request.question
        answer = None
        summary = None
        knowledge_points: list[str] = []
        if request.analysis is not None:
            answer = request.analysis.answer
            summary = request.analysis.summary
            knowledge_points = request.analysis.knowledgePoints
        if not answer:
            answer = question.suggestedAnswer or "暂未确定"

        focus = "、".join(knowledge_points[:3]) if knowledge_points else "题干条件与选项差异"
        summary_text = summary or "先定位题干中的限定条件，再逐项核对选项。"
        history_hint = f"这是当前题目的第 {len(request.history) + 1} 轮追问。"
        return (
            f"围绕这道题，{history_hint}关键关注点是：{focus}。"
            f"参考答案为 {answer}。{summary_text}"
            f"你问的是“{request.message}”，建议先复述题干条件，再说明对应选项为什么成立或不成立。"
        )

    def _build_system_prompt(self, request: ChatRequest) -> str:
        q = request.question

        if q.options:
            option_lines = []
            for opt in q.options:
                option_lines.append(f"  {opt['key'] if isinstance(opt, dict) else opt.key}. {opt['content'] if isinstance(opt, dict) else opt.content}")
            options_section = "- 选项：\n" + "\n".join(option_lines) + "\n"
        else:
            options_section = ""

        user_answer = request.userAnswer or "未作答"

        if request.analysis:
            a = request.analysis
            parts = []
            if a.answer:
                parts.append(f"- 正确答案：{a.answer}")
            if a.summary:
                parts.append(f"- 解析摘要：{a.summary}")
            if a.knowledgePoints:
                parts.append(f"- 知识点：{', '.join(a.knowledgePoints)}")
            if a.steps:
                parts.append(f"- 解题步骤：{'；'.join(a.steps)}")
            if a.optionAnalysis:
                opt_lines = [f"  {k}：{v}" for k, v in a.optionAnalysis.items()]
                parts.append(f"- 选项分析：\n" + "\n".join(opt_lines))
            if a.pitfalls:
                parts.append(f"- 常见陷阱：{', '.join(a.pitfalls)}")
            if a.reviewAdvice:
                parts.append(f"- 复习建议：{', '.join(a.reviewAdvice)}")
            analysis_section = "\n".join(parts)
        else:
            analysis_section = "暂未进行 AI 解析"

        return SYSTEM_PROMPT.format(
            stem=q.stem if isinstance(q, dict) else q.stem,
            questionType=q.questionType if isinstance(q, dict) else q.questionType,
            options_section=options_section,
            user_answer=user_answer,
            analysis_section=analysis_section,
        )
