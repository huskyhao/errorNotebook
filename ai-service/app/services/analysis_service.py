from __future__ import annotations

import asyncio
import json
import logging
import re
import time
from typing import Callable, TypeVar

from pydantic import ValidationError

from app.core.config import settings
from app.core.logging import format_log
from app.schemas.analysis import AnalysisPayload, AnalysisRequest, AnalysisResponse, TaxonomySuggestion
from app.services.openai_client import OpenAICompatibleError, build_openai_client
from app.services.vision_service import build_multimodal_client

logger = logging.getLogger("app.services.analysis")
T = TypeVar("T")

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
  "taxonomySuggestionReason": "没有匹配或候选不足时说明原因",
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
        fallback_warnings: list[str] = []

        if use_multimodal:
            try:
                analysis, completion_tokens = await asyncio.wait_for(
                    self._analyze_with_multimodal(payload, image_bytes, media_type),
                    timeout=settings.ai_action_timeout_seconds,
                )
            except (OpenAICompatibleError, asyncio.TimeoutError, TimeoutError) as exc:
                # A configured vision endpoint can be temporarily unavailable
                # while the text endpoint is healthy. OCR already produced a
                # structured question, so keep the import usable by falling
                # back to text analysis instead of returning mock content or
                # losing the question altogether.
                if self._client is None:
                    if isinstance(exc, OpenAICompatibleError):
                        raise
                    raise OpenAICompatibleError(
                        "PROVIDER_TIMEOUT", "multimodal provider request timed out", True, 504
                    ) from exc
                provider_error = exc if isinstance(exc, OpenAICompatibleError) else OpenAICompatibleError(
                    "PROVIDER_TIMEOUT", "multimodal provider request timed out", True, 504
                )
                logger.warning(
                    format_log(
                        "analysis.multimodal_fallback",
                        trace_id=payload.traceId,
                        question_id=payload.questionId,
                        code=provider_error.code,
                    )
                )
                fallback_warnings.append("multimodal_fallback_to_ocr")
                analysis, completion_tokens = await self._analyze_with_openai(payload)
        elif self._client:
            analysis, completion_tokens = await self._analyze_with_openai(payload)
        else:
            await asyncio.sleep(settings.llm_mock_delay_seconds)
            analysis = self._mock_analysis(payload)
            completion_tokens = 0

        llm_ms = int((time.perf_counter() - started_at) * 1000)
        analysis, warnings = self._validate_domain(analysis, payload)
        warnings = list(dict.fromkeys(fallback_warnings + warnings))
        response = AnalysisResponse(
            traceId=payload.traceId,
            questionId=payload.questionId,
            status="needs_review" if warnings else "completed",
            analysis=analysis,
            cost={"llmMs": llm_ms, "completionTokens": completion_tokens},
            warnings=warnings,
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
                result_status=response.status,
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

    @staticmethod
    def _validate_domain(analysis: AnalysisPayload, payload: AnalysisRequest) -> tuple[AnalysisPayload, list[str]]:
        warnings: list[str] = []
        option_keys = {item.key for item in payload.question.options}
        if option_keys:
            if analysis.answer not in option_keys:
                warnings.append("answer_not_in_options")
                analysis.answer = ""
            invalid_keys = [key for key in analysis.optionAnalysis if key not in option_keys]
            if invalid_keys:
                warnings.append("option_analysis_contains_unknown_option")
                analysis.optionAnalysis = {key: value for key, value in analysis.optionAnalysis.items() if key in option_keys}
        elif analysis.optionAnalysis:
            warnings.append("option_analysis_without_options")
            analysis.optionAnalysis = {}
        if any(item.startswith("missing_options_") for item in payload.question.warnings) or "options_incomplete" in payload.question.warnings:
            warnings.append("question_structure_incomplete")
        if not analysis.answer:
            warnings.append("answer_unconfirmed")
        suggestion = analysis.taxonomySuggestion
        if suggestion is not None:
            category_by_key = {
                item.strip().casefold(): item.strip()
                for item in payload.context.get("categoryCandidates", [])
                if isinstance(item, str) and item.strip()
            }
            if suggestion.categoryName and suggestion.categoryName.strip().casefold() not in category_by_key:
                suggestion.categoryName = None
                warnings.append("taxonomy_category_outside_candidates")
            elif suggestion.categoryName:
                suggestion.categoryName = category_by_key[suggestion.categoryName.strip().casefold()]
            clean_tags: list[str] = []
            seen_tags: set[str] = set()
            existing_tag_keys = {
                item.strip().casefold()
                for item in payload.context.get("tagCandidates", [])
                if isinstance(item, str) and item.strip()
            }
            for raw_tag in suggestion.tagNames:
                tag = raw_tag.strip()
                key = tag.casefold()
                if (len(tag) < 2 and key not in existing_tag_keys) or len(tag) > 32 or any(char in tag for char in "\r\n\t") or key in seen_tags:
                    continue
                seen_tags.add(key)
                clean_tags.append(tag)
                if len(clean_tags) == 3:
                    break
            suggestion.tagNames = clean_tags
            if suggestion.confidence is not None:
                suggestion.confidence = min(1, max(0, suggestion.confidence))
            if not suggestion.categoryName and not suggestion.tagNames:
                analysis.taxonomySuggestionReason = "没有可确认的大类学科或题面信息不足。"
            elif suggestion.categoryName is None:
                analysis.taxonomySuggestionReason = analysis.taxonomySuggestionReason or "未命中可用的大类学科，知识点标签仅供参考。"
        return analysis, list(dict.fromkeys(warnings))

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
            "taxonomySuggestion 只能返回建议：categoryName 必须从 context.categoryCandidates 中选择一个稳定的高层学科分类；"
            "tagNames 返回 1 到 3 个简短、具体的知识点标签，优先复用 context.tagCandidates，也允许提出新的知识点标签；"
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

        analysis, completion_tokens = await self._call_structured_with_budget(
            system_prompt=system_prompt, user_prompt=user_prompt, trace_id=payload.traceId,
            parser=lambda content: AnalysisPayload.model_validate(json.loads(_extract_json(content))),
        )
        return analysis, completion_tokens

    async def _call_structured_with_budget(self, *, system_prompt: str, user_prompt: str, trace_id: str, parser: Callable[[str], T]) -> tuple[T, int]:
        assert self._client is not None
        attempts = 0
        repair_used = False
        current_prompt = user_prompt
        while attempts < max(1, settings.ai_max_attempts):
            attempts += 1
            try:
                content, tokens = await asyncio.wait_for(
                    self._client.create_structured_completion(system_prompt=system_prompt, user_prompt=current_prompt),
                    timeout=settings.ai_action_timeout_seconds,
                )
                return parser(content), tokens
            except OpenAICompatibleError as exc:
                if not exc.retryable or attempts >= max(1, settings.ai_max_attempts):
                    raise
                await asyncio.sleep(min(0.25 * (2 ** (attempts - 1)), 1.0))
            except (asyncio.TimeoutError, TimeoutError) as exc:
                if attempts >= max(1, settings.ai_max_attempts):
                    raise OpenAICompatibleError("PROVIDER_TIMEOUT", "AI provider request timed out", True, 504) from exc
                await asyncio.sleep(min(0.25 * (2 ** (attempts - 1)), 1.0))
            except (json.JSONDecodeError, ValidationError, TypeError, ValueError) as exc:
                if repair_used or attempts >= max(1, settings.ai_max_attempts):
                    raise OpenAICompatibleError("INVALID_OUTPUT", "LLM response failed structured validation", True, 502) from exc
                repair_used = True
                current_prompt = user_prompt + f"\n\n仅修复上次 JSON 格式（错误：{str(exc)[:160]}），不要补造题面事实。输出格式：\n{_ANALYSIS_JSON_TEMPLATE}"
        raise OpenAICompatibleError("PROVIDER_UNAVAILABLE", f"provider call budget exhausted: {trace_id}", True, 503)

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
            "taxonomySuggestion.categoryName 必须从 context.categoryCandidates 中选择一个大类学科；"
            "tagNames 返回 1 到 3 个细粒度知识点，优先复用已有候选，也可提出新标签。"
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
                    content_len=len(content),
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
            taxonomySuggestion=TaxonomySuggestion(
                categoryName=(payload.context.get("categoryCandidates") or [None])[0],
                tagNames=list(payload.context.get("tagCandidates") or [])[:2],
                confidence=0.6 if payload.context.get("categoryCandidates") or payload.context.get("tagCandidates") else None,
            ),
            taxonomySuggestionReason=("AI 已自动完成分类和标签；用户可在题目详情中修改。" if payload.context.get("categoryCandidates") or payload.context.get("tagCandidates") else "没有可用的分类候选。"),
            reviewAdvice=[
                "优先整理同类题型的判断标准。",
                "对 OCR 不清晰或带图题，先人工校准后再看解析。",
            ],
        )
