from __future__ import annotations

import asyncio
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
    base_url: str
    api_key: str
    model: str
    timeout_seconds: int
    temperature: float
    max_tokens: int

    async def create_structured_completion_with_image(
        self, *, image_bytes: bytes, media_type: str, system_prompt: str, user_prompt: str,
    ) -> tuple[str, int]:
        return await self.create_chat_completion_with_images(
            images=[(image_bytes, media_type)], system_prompt=system_prompt, user_prompt=user_prompt
        )

    async def create_chat_completion_with_images(
        self, *, images: list[tuple[bytes, str]], system_prompt: str, user_prompt: str,
    ) -> tuple[str, int]:
        return await asyncio.to_thread(
            self._request, images=images, system_prompt=system_prompt, user_prompt=user_prompt,
        )

    def _request(self, *, images: list[tuple[bytes, str]], system_prompt: str, user_prompt: str) -> tuple[str, int]:
        image_parts = [
            {"type": "image_url", "image_url": {"url": f"data:{media_type};base64,{base64.b64encode(image_bytes).decode('ascii')}"}}
            for image_bytes, media_type in images
        ]
        payload = {
            "model": self.model, "temperature": self.temperature, "max_tokens": self.max_tokens,
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": image_parts + [{"type": "text", "text": user_prompt}]},
            ],
        }
        url = f"{self.base_url}/chat/completions"
        logger.info(format_log("multimodal.request", model=self.model, image_count=len(images), image_size_bytes=sum(len(item[0]) for item in images)))
        request = urllib.request.Request(
            url, data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
            headers={"Content-Type": "application/json", "Authorization": f"Bearer {self.api_key}"}, method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                response_bytes = response.read()
        except urllib.error.HTTPError as exc:
            exc.read()
            code = "PROVIDER_RATE_LIMIT" if exc.code == 429 else ("PROVIDER_AUTH" if exc.code in {401, 403} else "PROVIDER_UNAVAILABLE")
            raise OpenAICompatibleError(code, f"multimodal provider returned HTTP {exc.code}", exc.code == 429 or exc.code >= 500, 503) from exc
        except (urllib.error.URLError, TimeoutError) as exc:
            raise OpenAICompatibleError("PROVIDER_TIMEOUT", "multimodal provider request timed out or was unreachable", True, 504) from exc
        try:
            body = json.loads(response_bytes.decode("utf-8"))
            content = body["choices"][0]["message"]["content"]
            if not isinstance(content, str):
                raise TypeError("content is not text")
            usage = body.get("usage") or {}
            tokens = usage.get("completion_tokens")
            completion_tokens = int(tokens) if tokens is not None else 0
            logger.info(format_log("multimodal.response", model=self.model, completion_tokens=completion_tokens))
            return content, completion_tokens
        except (KeyError, IndexError, TypeError, ValueError, json.JSONDecodeError) as exc:
            raise OpenAICompatibleError("INVALID_OUTPUT", "multimodal provider returned an invalid response shape", True, 502) from exc


def build_multimodal_client() -> MultimodalClient | None:
    if not settings.vision_enabled:
        return None
    return MultimodalClient(
        base_url=settings.vision_base_url, api_key=settings.vision_api_key, model=settings.vision_model,
        timeout_seconds=settings.vision_timeout_seconds, temperature=settings.vision_temperature,
        max_tokens=settings.vision_max_tokens,
    )
