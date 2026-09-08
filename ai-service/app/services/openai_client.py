from __future__ import annotations

import asyncio
import json
import logging
import urllib.error
import urllib.request
from dataclasses import dataclass

from app.core.config import settings
from app.core.logging import format_log
from app.services.ai_errors import AIServiceError

logger = logging.getLogger("app.services.openai_client")


class OpenAICompatibleError(AIServiceError):
    """Backward-compatible error with safe defaults for older callers."""

    def __init__(self, code_or_message: str, message: str | None = None, retryable: bool = False, status_code: int = 502):
        if message is None:
            lowered = code_or_message.lower()
            code = "INVALID_OUTPUT" if "invalid" in lowered or "json" in lowered else "PROVIDER_UNAVAILABLE"
            message = code_or_message
        else:
            code = code_or_message
        super().__init__(code=code, message=message, retryable=retryable, status_code=status_code)


@dataclass(frozen=True)
class OpenAICompatibleClient:
    base_url: str
    api_key: str
    model: str
    timeout_seconds: int
    temperature: float
    max_tokens: int

    async def create_structured_completion(self, *, system_prompt: str, user_prompt: str) -> tuple[str, int]:
        return await self._create_completion(
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            structured=True,
        )

    async def create_chat_completion(self, *, messages: list[dict]) -> tuple[str, int]:
        return await self._create_completion(messages=messages, structured=False)

    async def _create_completion(self, *, messages: list[dict], structured: bool) -> tuple[str, int]:
        # urllib remains compatible with OpenAI-compatible endpoints, but runs in a
        # worker thread so the async server event loop is never blocked.
        return await asyncio.to_thread(self._request, messages, structured)

    def _request(self, messages: list[dict], structured: bool) -> tuple[str, int]:
        url = f"{self.base_url}/chat/completions"
        payload: dict[str, object] = {
            "model": self.model,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
            "messages": messages,
        }
        if structured:
            payload["response_format"] = {"type": "json_object"}
        logger.info(format_log("provider.request", provider="openai_compatible", model=self.model, timeout_seconds=self.timeout_seconds))
        request = urllib.request.Request(
            url,
            data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
            headers={"Content-Type": "application/json", "Authorization": f"Bearer {self.api_key}"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                response_bytes = response.read()
        except urllib.error.HTTPError as exc:
            code = "PROVIDER_RATE_LIMIT" if exc.code == 429 else (
                "PROVIDER_AUTH" if exc.code in {401, 403} else "PROVIDER_UNAVAILABLE"
            )
            retryable = exc.code == 429 or exc.code >= 500
            raise OpenAICompatibleError(code, f"AI provider returned HTTP {exc.code}", retryable, 503 if retryable else 502) from exc
        except (urllib.error.URLError, TimeoutError) as exc:
            raise OpenAICompatibleError("PROVIDER_TIMEOUT", "AI provider request timed out or was unreachable", True, 504) from exc
        try:
            body = json.loads(response_bytes.decode("utf-8"))
            content = body["choices"][0]["message"]["content"]
            if not isinstance(content, str):
                raise TypeError("content is not text")
            usage = body.get("usage") or {}
            tokens = usage.get("completion_tokens")
            completion_tokens = int(tokens) if tokens is not None else 0
            logger.info(format_log("provider.response", provider="openai_compatible", model=self.model, completion_tokens=completion_tokens))
            return content, completion_tokens
        except (KeyError, IndexError, TypeError, ValueError, json.JSONDecodeError) as exc:
            raise OpenAICompatibleError("INVALID_OUTPUT", "AI provider returned an invalid response shape", True, 502) from exc


def build_openai_client() -> OpenAICompatibleClient | None:
    if not settings.openai_enabled:
        return None
    return OpenAICompatibleClient(
        base_url=settings.openai_base_url,
        api_key=settings.openai_api_key,
        model=settings.openai_model,
        timeout_seconds=settings.openai_timeout_seconds,
        temperature=settings.openai_temperature,
        max_tokens=settings.openai_max_tokens,
    )
