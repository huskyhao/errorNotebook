from __future__ import annotations

import base64
import json
import logging
import urllib.error
import urllib.request
from dataclasses import dataclass

from app.core.config import settings
from app.core.logging import format_log
from app.services.openai_client import OpenAICompatibleError

logger = logging.getLogger("app.services.vision")


@dataclass(frozen=True)
class MultimodalClient:
    """Generic multimodal LLM client that sends image + text and returns structured JSON."""

    base_url: str
    api_key: str
    model: str
    timeout_seconds: int
    temperature: float
    max_tokens: int

    async def create_structured_completion_with_image(
        self, *, image_bytes: bytes, media_type: str,
        system_prompt: str, user_prompt: str,
    ) -> tuple[str, int]:
        image_b64 = base64.b64encode(image_bytes).decode("ascii")
        data_url = f"data:{media_type};base64,{image_b64}"

        url = f"{self.base_url}/chat/completions"
        payload = {
            "model": self.model,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
            "messages": [
                {"role": "system", "content": system_prompt},
                {
                    "role": "user",
                    "content": [
                        {"type": "image_url", "image_url": {"url": data_url}},
                        {"type": "text", "text": user_prompt},
                    ],
                },
            ],
        }

        logger.info(
            format_log(
                "multimodal.request",
                model=self.model,
                url=url,
                media_type=media_type,
                image_size_bytes=len(image_bytes),
            )
        )

        request = urllib.request.Request(
            url,
            data=json.dumps(payload).encode("utf-8"),
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {self.api_key}",
            },
            method="POST",
        )

        try:
            with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                response_bytes = response.read()
        except urllib.error.HTTPError as exc:
            body = exc.read().decode("utf-8", errors="replace")
            logger.error(
                format_log(
                    "multimodal.http_error",
                    model=self.model,
                    url=url,
                    status_code=exc.code,
                    response_body=body[:500],
                )
            )
            raise OpenAICompatibleError(f"multimodal api returned {exc.code}: {body}") from exc
        except urllib.error.URLError as exc:
            logger.exception(
                format_log(
                    "multimodal.network_error",
                    model=self.model,
                    url=url,
                    reason=exc.reason,
                )
            )
            raise OpenAICompatibleError(f"multimodal api request failed: {exc.reason}") from exc

        try:
            body = json.loads(response_bytes.decode("utf-8"))
            content = body["choices"][0]["message"]["content"]
            usage = body.get("usage", {})
            prompt_tokens = int(usage.get("prompt_tokens", 0))
            completion_tokens = int(usage.get("completion_tokens", 0))
            logger.info(
                format_log(
                    "multimodal.response",
                    model=self.model,
                    prompt_tokens=prompt_tokens,
                    completion_tokens=completion_tokens,
                    content_len=len(content),
                )
            )
            return content, completion_tokens
        except (KeyError, IndexError, TypeError, ValueError, json.JSONDecodeError) as exc:
            logger.exception(
                format_log(
                    "multimodal.invalid_response",
                    model=self.model,
                    url=url,
                )
            )
            raise OpenAICompatibleError("invalid multimodal api response shape") from exc


def build_multimodal_client() -> MultimodalClient | None:
    if not settings.vision_enabled:
        return None

    return MultimodalClient(
        base_url=settings.vision_base_url,
        api_key=settings.vision_api_key,
        model=settings.vision_model,
        timeout_seconds=settings.vision_timeout_seconds,
        temperature=settings.vision_temperature,
        max_tokens=settings.vision_max_tokens,
    )
