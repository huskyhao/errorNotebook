from __future__ import annotations

import asyncio
import json
import logging
import re
import time

from pydantic import ValidationError

from app.core.config import settings
from app.core.logging import format_log
from app.schemas.analysis import AnalysisPayload, AnalysisRequest, AnalysisResponse
from app.services.openai_client import OpenAICompatibleError, build_openai_client
from app.services.vision_service import MultimodalClient, build_multimodal_client

logger = logging.getLogger("app.services.analysis")

_ANALYSIS_JSON_TEMPLATE = """\
{
  "answer": "正确选项",
  "summary": "题目考查的核心知识点概述",
  "knowledgePoints": ["知识点1", "知识点2"],
  "taxonomySuggestion": {
    "categoryName": "稳定的高层学科分类；无法确定时留空",
    "tagNames": ["细粒度知识点1", "细粒度知识点2"],
    "confidence": 0.0
  },
  "steps": ["步骤1：分析题干条件和约束", "步骤2：逐项比对选项"],
  "optionAnalysis": {
    "A": "选项A的分析（为什么对/错）",
    "B": "选项B的分析（为什么对/错）"
  },
  "pitfalls": ["常见错误1", "常见错误2"],
  "reviewAdvice": ["复习建议1", "复习建议2"]
}"""


def _extract_json(text: str) -> str:
    """Extract JSON from model output that may be wrapped in markdown or contain extra text."""
    text = text.strip()
    if not text:
        return text

    # 1. Try markdown code block: ```json ... ``` or ``` ... ```
    md_match = re.search(r"```(?:json)?\s*\n(.*?)\n\s*```", text, re.DOTALL)
    if md_match:
        extracted = md_match.group(1).strip()
        logger.info("analysis.json_extracted_from_markdown | method=markdown_block")
        return extracted

    # 2. Try to find JSON object boundaries: first { to last }
    start = text.find("{")
    end = text.rfind("}")
    if start != -1 and end != -1 and end > start:
        extracted = text[start:end + 1]
        if extracted != text:
            logger.info("analysis.json_extracted_from_boundaries | method=brace_boundaries")
        return extracted

    return text


class AnalysisService:
    def __init__(self) -> None:
        self._client = build_openai_client()
        self._multimodal_client = build_multimodal_client()

    @property
    def backend_name(self) -> str:
        return "openai_compatible" if self._client else "mock"

    @property
    def multimodal_backend_name(self) -> str:
        return self._multimodal_client.model if self._multimodal_client else "none"

    async def analyze_question(
        self,
        payload: AnalysisRequest,
        image_bytes: bytes | None = None,
        media_type: str | None = None,
    ) -> AnalysisResponse:
        use_multimodal = image_bytes is not None and self._multimodal_client is not None
        backend_label = self.multimodal_backend_name if use_multimodal else self.backend_name

        logger.info(
            format_log(
                "analysis.started",
                trace_id=payload.traceId,
                question_id=payload.questionId,
                backend=backend_label,
                question_type=payload.question.questionType,
                option_count=len(payload.question.options),
                has_user_answer=payload.userAnswer is not None,
                has_image=image_bytes is not None,
                use_multimodal=use_multimodal,
            )
        )
        started_at = time.perf_counter()

        if use_multimodal:
            analysis, completion_tokens = await self._analyze_with_multimodal(
                payload, image_bytes, media_type
            )
        elif self._client:
            analysis, completion_tokens = await self._analyze_with_openai(payload)
        else:
            await asyncio.sleep(settings.llm_mock_delay_seconds)
            analysis = self._mock_analysis(payload)
            completion_tokens = 0

        llm_ms = int((time.perf_counter() - started_at) * 1000)
        response = AnalysisResponse(
            traceId=payload.traceId,
            questionId=payload.questionId,
            status="completed",
            analysis=analysis,
            cost={"llmMs": llm_ms, "completionTokens": completion_tokens},
        )
        logger.info(
            format_log(
                "analysis.completed",
                trace_id=payload.traceId,
                question_id=payload.questionId,
                backend=backend_label,
                llm_ms=llm_ms,
                completion_tokens=completion_tokens,
                answer=analysis.answer,
            )
        )
        logger.info(
            format_log(
                "analysis.result",
                trace_id=payload.traceId,
                question_id=payload.questionId,
                answer=analysis.answer,
                summary=analysis.summary,
                knowledge_points=analysis.knowledgePoints,
                steps_count=len(analysis.steps),
                option_analysis_keys=list(analysis.optionAnalysis.keys()),
                pitfalls_count=len(analysis.pitfalls),
            )
        )
        return response

    async def _analyze_with_openai(self, payload: AnalysisRequest) -> tuple[AnalysisPayload, int]:
        assert self._client is not None
        system_prompt = (
            "你是 ErroNotebook 的题目解析助手。"
            "你必须严格按照下面所示的 JSON 格式返回题目解析，直接输出花括号开始的 JSON 对象，"
            "不要输出 markdown 代码块、不要输出任何解释性文字。\n\n"
            f"```json\n{_ANALYSIS_JSON_TEMPLATE}\n```\n\n"
            "如果题目无法完全确定，也要给出保守但结构合法的 JSON。"
            "如果 question.warnings 或 context.structureWarnings 提示 options_incomplete、missing_options_*、llm_refine_failed，"
            "说明题面结构可能不完整；此时必须参考 question.rawText，不要只依据 options 数组判断题目。"
            "taxonomySuggestion 只能返回建议：categoryName 必须是稳定的高层学科分类，tagNames 只能是细粒度知识点；"
            "不要把 TCP、UDP 等知识点放进 categoryName，也不要声称建议已经生效。"
        )
        user_prompt = json.dumps(
            {
                "questionId": payload.questionId,
                "traceId": payload.traceId,
                "question": payload.question.model_dump(),
                "userAnswer": payload.userAnswer,
                "context": payload.context,
            },
            ensure_ascii=False,
        )

        content, completion_tokens = await self._client.create_structured_completion(
            system_prompt=system_prompt,
            user_prompt=user_prompt,
        )

        logger.info(
            format_log(
                "analysis.raw_response",
                trace_id=payload.traceId,
                question_id=payload.questionId,
                backend=self.backend_name,
                content_len=len(content),
                content=content,
            )
        )

        extracted = _extract_json(content)
        try:
            return AnalysisPayload.model_validate(json.loads(extracted)), completion_tokens
        except (json.JSONDecodeError, ValidationError) as exc:
            logger.error(
                format_log(
                    "analysis.invalid_response",
                    trace_id=payload.traceId,
                    question_id=payload.questionId,
                    backend=self.backend_name,
                    raw_content=content[:500],
                    extracted=extracted[:500],
                )
            )
            raise OpenAICompatibleError("llm response is not valid structured json") from exc

    async def _analyze_with_multimodal(
        self, payload: AnalysisRequest, image_bytes: bytes, media_type: str
    ) -> tuple[AnalysisPayload, int]:
        assert self._multimodal_client is not None
        system_prompt = (
            "你是 ErroNotebook 的题目解析助手，具备图像理解能力。"
            "你会收到一张题目图片和 OCR 提取的文字（可能包含错误）。"
            "请仔细查看图片中的题目内容（包括图形、图表、公式、表格等视觉元素），"
            "结合 OCR 文字参考，给出完整的题目解析。"
            "你必须严格按照下面所示的 JSON 格式返回题目解析，直接输出花括号开始的 JSON 对象，"
            "不要输出 markdown 代码块、不要输出任何解释性文字。\n\n"
            f"```json\n{_ANALYSIS_JSON_TEMPLATE}\n```\n\n"
            "如果题目无法完全确定，也要给出保守但结构合法的 JSON。"
            "如果 question.warnings 或 context.structureWarnings 提示 options_incomplete、missing_options_*、llm_refine_failed，"
            "说明题面结构可能不完整；此时必须结合图片和 question.rawText，不要只依据 options 数组判断题目。"
        )
        question_text = json.dumps(
            payload.question.model_dump(), ensure_ascii=False
        )
        user_prompt = (
            f"以下是一道题目的 OCR 识别文字（可能存在识别误差，以图片为准）：\n```json\n{question_text}\n```\n\n"
            f"用户的作答：{payload.userAnswer or '未作答'}\n\n"
            "请根据题目图片和以上文字参考，给出完整的题目解析。"
        )

        content, completion_tokens = await self._multimodal_client.create_structured_completion_with_image(
            system_prompt=system_prompt,
            user_prompt=user_prompt,
            image_bytes=image_bytes,
            media_type=media_type,
        )

        logger.info(
            format_log(
                "analysis.raw_response",
                trace_id=payload.traceId,
                question_id=payload.questionId,
                backend=self.multimodal_backend_name,
                content_len=len(content),
                content=content,
            )
        )

        extracted = _extract_json(content)
        try:
            return AnalysisPayload.model_validate(json.loads(extracted)), completion_tokens
        except (json.JSONDecodeError, ValidationError) as exc:
            logger.error(
                format_log(
                    "analysis.invalid_multimodal_response",
                    trace_id=payload.traceId,
                    question_id=payload.questionId,
                    backend=self.multimodal_backend_name,
                    raw_content=content[:500],
                    extracted=extracted[:500],
                )
            )
            raise OpenAICompatibleError("multimodal llm response is not valid structured json") from exc

    @staticmethod
    def _mock_analysis(payload: AnalysisRequest) -> AnalysisPayload:
        suggested = payload.question.suggestedAnswer or (
            payload.question.options[0].key if payload.question.options else "unknown"
        )
        return AnalysisPayload(
            answer=suggested,
            summary="当前为 mock 解析结果。建议重点检查题干约束、选项差异以及并发/数据结构等核心知识点。",
            knowledgePoints=["题干约束识别", "选项比较", "408 常见考点归纳"],
            steps=[
                "先定位题干中的限定条件和关键词。",
                "逐项比较各选项是否满足题干约束。",
                "结合 408 常见考点给出结论与复习建议。",
            ],
            optionAnalysis={
                option.key: f"请结合题干重新核对选项 {option.key} 的条件是否成立。"
                for option in payload.question.options
            },
            pitfalls=[
                "把互斥和同步混为同一概念。",
                "忽略题干里的前提条件或适用范围。",
            ],
            reviewAdvice=[
                "优先整理同类题型的判断标准。",
                "对 OCR 不清晰或带图题，先人工校准后再看解析。",
            ],
        )
