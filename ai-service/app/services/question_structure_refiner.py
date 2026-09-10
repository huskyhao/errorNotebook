from __future__ import annotations

import json
import logging
import re
from typing import Protocol

from pydantic import ValidationError

from app.core.logging import format_log
from app.schemas.question import OptionItem, QuestionMetadata, StructuredQuestion
from app.services.openai_client import OpenAICompatibleError, build_openai_client
from app.services.question_structurer import StructuredQuestionResult, strip_question_ui_prefix

logger = logging.getLogger("app.services.question_structure_refiner")


class StructuredCompletionClient(Protocol):
    async def create_structured_completion(self, *, system_prompt: str, user_prompt: str) -> tuple[str, int]:
        ...


_REFINE_JSON_TEMPLATE = """\
{
  "stem": "只包含题干，不包含任何选项内容",
  "questionType": "single_choice",
  "options": [
    {"key": "A", "content": "选项A内容"},
    {"key": "B", "content": "选项B内容"}
  ],
  "warnings": ["options_incomplete"]
}"""


def _extract_json(text: str) -> str:
    text = text.strip()
    match = re.search(r"```(?:json)?\s*\n(.*?)\n\s*```", text, re.DOTALL)
    if match:
        return match.group(1).strip()
    start = text.find("{")
    end = text.rfind("}")
    if start != -1 and end != -1 and end > start:
        return text[start:end + 1]
    return text


class QuestionStructureRefiner:
    def __init__(self, client: StructuredCompletionClient | None = None, use_default_client: bool = True) -> None:
        self._client = client if client is not None else (build_openai_client() if use_default_client else None)

    async def refine(
        self,
        *,
        source_type: str,
        filename: str,
        result: StructuredQuestionResult,
    ) -> StructuredQuestionResult:
        if not self._should_refine(result):
            return result

        if self._client is None:
            return self._fallback(result, "llm_refine_unavailable")

        system_prompt = (
            "你是 ErroNotebook 的 OCR 题面结构校验器，只负责修正题干和选项边界。"
            "你不能解题，不能生成答案解析，不能根据知识推断正确答案。"
            "你必须严格根据 rawOcrText 和规则解析结果恢复题面结构。"
            "如果 rawOcrText 中确实看不到某个选项内容，不要编造，只在 warnings 中标记。"
            "题干必须移除倒计时、题目进度、题型、分值、难度等考试界面信息。"
            "识别 OA.、O B.、D。 等 OCR 噪声；如果 B 与 D 之间有明显选项语义但缺少 C 标记，可恢复为 C。"
            "直接返回 JSON 对象，不要 markdown，不要解释。\n\n"
            f"JSON 示例：\n{_REFINE_JSON_TEMPLATE}"
        )
        user_prompt = json.dumps(
            {
                "sourceType": source_type,
                "filename": filename,
                "rawOcrText": result.raw_text,
                "ruleParsed": result.structured_question.model_dump(),
                "ruleWarnings": result.warnings,
            },
            ensure_ascii=False,
        )

        try:
            content, _ = await self._client.create_structured_completion(
                system_prompt=system_prompt,
                user_prompt=user_prompt,
            )
            refined = self._build_structured_question(
                payload=json.loads(_extract_json(content)),
                base=result.structured_question,
                source_type=source_type,
            )
        except (OpenAICompatibleError, json.JSONDecodeError, TypeError, ValidationError, ValueError, RuntimeError) as exc:
            logger.warning(
                format_log(
                    "structure_refine.failed",
                    filename=filename,
                    error=str(exc),
                )
            )
            return self._fallback(result, "llm_refine_failed")

        warnings = self._merge_warnings(result.warnings, refined.warnings, refined.options)
        refined.warnings = warnings
        refined.rawText = result.raw_text
        refined.metadata.extractionMethod = "llm_refined"
        refined.metadata.ocrConfidence = self._refined_confidence(result, warnings)

        logger.info(
            format_log(
                "structure_refine.completed",
                filename=filename,
                option_count=len(refined.options),
                warnings=warnings,
            )
        )
        return StructuredQuestionResult(
            raw_text=result.raw_text,
            structured_question=refined,
            warnings=warnings,
            parser_ms=result.parser_ms,
        )

    @staticmethod
    def _should_refine(result: StructuredQuestionResult) -> bool:
        warning_set = set(result.warnings)
        if any(item.startswith("missing_options_") for item in warning_set):
            return True
        if warning_set.intersection({"options_incomplete", "options_not_detected", "stem_missing", "ocr_option_noise_removed"}):
            return True
        raw = result.raw_text
        return bool(re.search(r"\b[Oo0〇○]\s*[A-D][\.、．:：。]", raw) or re.search(r"^[A-D]。", raw, re.MULTILINE))

    @staticmethod
    def _build_structured_question(*, payload: dict, base: StructuredQuestion, source_type: str) -> StructuredQuestion:
        options = [
            OptionItem(key=str(item.get("key", "")).strip(), content=str(item.get("content", "")).strip())
            for item in payload.get("options", [])
            if str(item.get("key", "")).strip() and str(item.get("content", "")).strip()
        ]
        warnings = [str(item).strip() for item in payload.get("warnings", []) if str(item).strip()]
        question_type = str(payload.get("questionType") or base.questionType)
        if question_type not in {"single_choice", "multiple_choice", "subjective", "unknown"}:
            question_type = base.questionType

        refined_stem, ui_noise_removed = strip_question_ui_prefix(
            str(payload.get("stem") or base.stem)
        )
        if ui_noise_removed:
            warnings = list(dict.fromkeys([*warnings, "ocr_exam_ui_noise_removed"]))

        return StructuredQuestion(
            stem=refined_stem,
            questionType=question_type,
            options=options,
            assets=base.assets,
            suggestedAnswer=base.suggestedAnswer,
            rawText=base.rawText,
            hasDiagram=base.hasDiagram,
            diagramDescription=base.diagramDescription,
            warnings=warnings,
            metadata=QuestionMetadata(
                sourceType=source_type,
                hasDiagram=base.metadata.hasDiagram,
                ocrConfidence=base.metadata.ocrConfidence,
                importMode=base.metadata.importMode,
                extractionMethod=base.metadata.extractionMethod,
            ),
        )

    @staticmethod
    def _merge_warnings(rule_warnings: list[str], refined_warnings: list[str], options: list[OptionItem]) -> list[str]:
        warnings = list(dict.fromkeys([*rule_warnings, *refined_warnings, "llm_refined"]))
        present = {item.key for item in options}
        missing = [key for key in ["A", "B", "C", "D"] if key not in present]
        warnings = [item for item in warnings if not item.startswith("missing_options_")]
        if options and len(options) < 4:
            if "options_incomplete" not in warnings:
                warnings.append("options_incomplete")
            if missing:
                warnings.append("missing_options_" + "".join(missing))
        elif len(options) >= 4:
            warnings = [item for item in warnings if item != "options_incomplete"]
        return warnings

    @staticmethod
    def _refined_confidence(result: StructuredQuestionResult, warnings: list[str]) -> float | None:
        confidence = result.structured_question.metadata.ocrConfidence
        if confidence is None:
            confidence = 0.86
        if any(item.startswith("missing_options_") for item in warnings) or "options_incomplete" in warnings:
            return min(confidence, 0.62)
        return max(confidence, 0.82)

    @staticmethod
    def _fallback(result: StructuredQuestionResult, warning: str) -> StructuredQuestionResult:
        warnings = list(dict.fromkeys([*result.warnings, warning]))
        question = result.structured_question
        question.warnings = warnings
        if any(item.startswith("missing_options_") for item in warnings) or "options_incomplete" in warnings:
            current = question.metadata.ocrConfidence
            question.metadata.ocrConfidence = 0.62 if current is None else min(current, 0.62)
        return StructuredQuestionResult(
            raw_text=result.raw_text,
            structured_question=question,
            warnings=warnings,
            parser_ms=result.parser_ms,
        )
