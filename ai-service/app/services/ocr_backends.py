from __future__ import annotations

import logging
import re
from dataclasses import dataclass
from typing import Protocol

from app.core.config import settings

logger = logging.getLogger("app.services.ocr_backends")


@dataclass(frozen=True)
class OCRTextBlock:
    text: str
    confidence: float


@dataclass(frozen=True)
class OCRBackendResult:
    blocks: list[OCRTextBlock]
    engine: str


class OCRBackend(Protocol):
    async def extract_text(self, *, file_bytes: bytes, filename: str, media_type: str) -> OCRBackendResult:
        ...


class MockOCRBackend:
    async def extract_text(self, *, file_bytes: bytes, filename: str, media_type: str) -> OCRBackendResult:
        del file_bytes, media_type
        sample = (
            f"题目来源: {filename}\n"
            "以下关于进程同步与互斥的说法，正确的是：\n"
            "A. 互斥和同步描述的是同一种约束\n"
            "B. 同步不需要考虑执行先后关系\n"
            "C. 互斥强调共享资源访问顺序\n"
            "D. 同步强调多个并发实体之间的协作关系"
        )
        return OCRBackendResult(
            blocks=[
                OCRTextBlock(text=line, confidence=0.94)
                for line in sample.splitlines()
                if line.strip()
            ],
            engine="mock",
        )


class PaddleOCRBackend:
    def __init__(self) -> None:
        from paddleocr import PaddleOCR  # type: ignore

        self._ocr = PaddleOCR(use_angle_cls=True, lang="ch")

    async def extract_text(self, *, file_bytes: bytes, filename: str, media_type: str) -> OCRBackendResult:
        del media_type
        import tempfile

        with tempfile.NamedTemporaryFile(suffix=_guess_suffix(filename), delete=False) as temp_file:
            temp_file.write(file_bytes)
            temp_path = temp_file.name

        try:
            result = self._ocr.ocr(temp_path)
        finally:
            try:
                import os

                os.remove(temp_path)
            except OSError:
                pass

        blocks: list[OCRTextBlock] = []
        for page in result or []:
            if isinstance(page, dict):
                rec_texts = page.get("rec_texts", []) or []
                rec_scores = page.get("rec_scores", []) or []
                for text, score in zip(rec_texts, rec_scores):
                    text = str(text).strip()
                    if text:
                        blocks.append(OCRTextBlock(text=text, confidence=float(score)))
            elif isinstance(page, list):
                for line in page:
                    try:
                        text = str(line[1][0]).strip()
                        confidence = float(line[1][1])
                    except (IndexError, TypeError, ValueError):
                        continue
                    if text:
                        blocks.append(OCRTextBlock(text=text, confidence=confidence))

        return OCRBackendResult(blocks=blocks, engine="paddleocr")


def build_ocr_backend() -> OCRBackend:
    requested = settings.ocr_backend.lower()
    if requested == "mock":
        return MockOCRBackend()

    if requested in {"auto", "paddleocr"}:
        try:
            return PaddleOCRBackend()
        except Exception:
            return MockOCRBackend()

    return MockOCRBackend()


def has_diagram_hint(raw_text: str) -> bool:
    patterns = [
        r"如图",
        r"下图",
        r"上图",
        r"图示",
        r"图中",
        r"示意图",
        r"流程图",
        r"结构图",
        r"原理图",
        r"架构图",
        r"框架图",
        r"时序图",
        r"状态图",
        r"电路图",
        r"曲线图",
        r"折线图",
        r"拓扑",
        r"二叉树",
        r"链表",
        r"网络图",
        r"有向图",
        r"无向图",
        r"邻接表",
        r"邻接矩阵",
        r"散列表",
        r"流水线",
        r"存储结构",
        r"地址映射",
        r"页表",
        r"指令流程",
        r"状态转换",
        r"状态机",
        r"总线结构",
    ]
    return any(re.search(pattern, raw_text) for pattern in patterns)


def _guess_suffix(filename: str) -> str:
    if "." in filename:
        return "." + filename.rsplit(".", 1)[1]
    return ".png"
