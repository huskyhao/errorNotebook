from __future__ import annotations

import re
from dataclasses import dataclass
from enum import Enum, auto

from app.schemas.question import OptionItem, QuestionAsset, QuestionMetadata, StructuredQuestion
from app.services.ocr_backends import OCRBackendResult, has_diagram_hint


# Matches option lines with common OCR noise, such as "OA.", "O B.5" or "D。以上都不正确".
_OPTION_LINE_PATTERN = re.compile(r"^\s*(?P<noise>[Oo0〇○]\s*)?(?P<key>[A-D])\s*(?P<sep>[\.、．:：。])?\s*(?P<content>.*)$")
_NOISE_ONLY_PATTERN = re.compile(r"^[Oo0〇○\.\s。．]+$")


class _ParseState(Enum):
    SCANNING = auto()
    PENDING_OPTION = auto()


@dataclass(frozen=True)
class StructuredQuestionResult:
    raw_text: str
    structured_question: StructuredQuestion
    warnings: list[str]
    parser_ms: int


class QuestionStructurer:
    def structure(
        self,
        *,
        source_type: str,
        filename: str,
        ocr_result: OCRBackendResult,
    ) -> StructuredQuestionResult:
        raw_text = "\n".join(block.text for block in ocr_result.blocks if block.text.strip()).strip()
        confidence = self._average_confidence(ocr_result)
        warnings: list[str] = []
        option_items: list[OptionItem] = []
        stem_lines: list[str] = []

        state = _ParseState.SCANNING
        pending_key = ""
        pending_content: list[str] = []

        for line in raw_text.splitlines():
            stripped = line.strip()
            if not stripped:
                continue

            option_match = self._match_option_line(stripped)
            if option_match:
                key, content = option_match
                if state == _ParseState.PENDING_OPTION:
                    option_items.append(self._build_option(pending_key, pending_content))
                    pending_content = []
                pending_key = key
                if content:
                    option_items.append(OptionItem(key=key, content=content))
                    pending_key = ""
                    state = _ParseState.SCANNING
                else:
                    state = _ParseState.PENDING_OPTION
                continue

            if self._is_noise_only(stripped):
                warnings.append("ocr_option_noise_removed")
                continue

            # Not an option line
            if state == _ParseState.PENDING_OPTION:
                pending_content.append(stripped)
            else:
                stem_lines.append(stripped)

        # Flush any pending option at end
        if state == _ParseState.PENDING_OPTION:
            option_items.append(self._build_option(pending_key, pending_content))

        question_type = "single_choice" if option_items else "subjective"
        option_items = [item for item in option_items if item.content.strip()]

        if len(option_items) == 0:
            warnings.append("options_not_detected")
        elif len(option_items) < 4:
            warnings.append("options_incomplete")
            missing = self._missing_option_keys(option_items)
            if missing:
                warnings.append("missing_options_" + "".join(missing))

        if confidence is not None and confidence < 0.72:
            warnings.append("ocr_low_confidence")

        has_diagram = has_diagram_hint(raw_text) or filename.lower().endswith((".svg", ".bmp"))
        assets: list[QuestionAsset] = []
        if has_diagram:
            assets.append(
                QuestionAsset(
                    type="image",
                    role="question_attachment",
                    ocrRelated=True,
                    sourceName=filename,
                    description="题面可能包含图形或结构化示意图",
                    metadata={"needsVisionFallback": True},
                )
            )
            warnings.append("diagram_detected")

        if not stem_lines:
            stem_lines = ["题面提取不完整，请人工校准"]
            warnings.append("stem_missing")

        structured_question = StructuredQuestion(
            stem="\n".join(stem_lines).strip(),
            questionType=question_type if question_type in {"single_choice", "subjective"} else "unknown",
            options=option_items,
            assets=assets,
            suggestedAnswer=None,
            rawText=raw_text,
            hasDiagram=has_diagram,
            warnings=warnings.copy(),
            metadata=QuestionMetadata(
                sourceType=source_type,
                hasDiagram=has_diagram,
                ocrConfidence=confidence,
                importMode="single_question",
                extractionMethod=ocr_result.engine,
            ),
        )

        return StructuredQuestionResult(
            raw_text=raw_text,
            structured_question=structured_question,
            warnings=warnings,
            parser_ms=12,
        )

    @staticmethod
    def _build_option(key: str, content_lines: list[str]) -> OptionItem:
        return OptionItem(key=key, content="\n".join(content_lines).strip())

    @staticmethod
    def _match_option_line(line: str) -> tuple[str, str] | None:
        match = _OPTION_LINE_PATTERN.match(line)
        if not match:
            return None
        has_noise = bool(match.group("noise"))
        has_sep = bool(match.group("sep"))
        content = match.group("content").strip()
        if not has_noise and not has_sep:
            return None
        if _NOISE_ONLY_PATTERN.fullmatch(content):
            content = ""
        return match.group("key"), content

    @staticmethod
    def _is_noise_only(line: str) -> bool:
        return bool(_NOISE_ONLY_PATTERN.fullmatch(line.strip()))

    @staticmethod
    def _missing_option_keys(options: list[OptionItem]) -> list[str]:
        present = {item.key for item in options}
        return [key for key in ["A", "B", "C", "D"] if key not in present]

    @staticmethod
    def _average_confidence(ocr_result: OCRBackendResult) -> float | None:
        if not ocr_result.blocks:
            return None
        return round(sum(block.confidence for block in ocr_result.blocks) / len(ocr_result.blocks), 4)
