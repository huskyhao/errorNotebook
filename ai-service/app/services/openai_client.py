from __future__ import annotations

import json
import logging
import urllib.error
import urllib.request
from dataclasses import dataclass

from app.core.config import settings
from app.core.logging import format_log

logger = logging.getLogger("app.services.openai_client")


class OpenAICompatibleError(RuntimeError):
    pass


@dataclass(frozen=True)
class OpenAICompatibleClient:
    base_url: str
    api_key: str
    model: str
    timeout_seconds: int
    temperature: float
    max_tokens: int

    async def create_structured_completion(self, *, system_prompt: str, user_prompt: str) -> tuple[str, int]:
        url = f"{self.base_url}/chat/completions"
        payload = {
            "model": self.model,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
            "response_format": {"type": "json_object"},
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
        }
        logger.info(
            format_log(
                "openai.request",
                model=self.model,
                url=url,
                timeout_seconds=self.timeout_seconds,
                max_tokens=self.max_tokens,
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
            logger.exception(
                format_log(
                    "openai.http_error",
                    model=self.model,
                    url=url,
                    status_code=exc.code,
                )
            )
            raise OpenAICompatibleError(f"openai-compatible api returned {exc.code}: {body}") from exc
        except urllib.error.URLError as exc:
            logger.exception(
                format_log(
                    "openai.network_error",
                    model=self.model,
                    url=url,
                    reason=exc.reason,
                )
            )
            raise OpenAICompatibleError(f"openai-compatible api request failed: {exc.reason}") from exc

        try:
            payload = json.loads(response_bytes.decode("utf-8"))
            content = payload["choices"][0]["message"]["content"]
            usage = payload.get("usage", {})
            completion_tokens = int(usage.get("completion_tokens", 0))
            logger.info(
                format_log(
                    "openai.response",
                    model=self.model,
                    completion_tokens=completion_tokens,
                )
            )
            return content, completion_tokens
        except (KeyError, IndexError, TypeError, ValueError, json.JSONDecodeError) as exc:
            logger.exception(
                format_log(
                    "openai.invalid_response",
                    model=self.model,
                    url=url,
                )
            )
            raise OpenAICompatibleError("invalid openai-compatible response shape") from exc


    async def create_chat_completion(self, *, messages: list[dict]) -> tuple[str, int]:
        """Send a free-form chat request without forced JSON mode."""
        url = f"{self.base_url}/chat/completions"
        payload = {
            "model": self.model,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
            "messages": messages,
        }
        logger.info(
            format_log(
                "openai.chat_request",
                model=self.model,
                url=url,
                message_count=len(messages),
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
            logger.exception(
                format_log(
                    "openai.http_error",
                    model=self.model,
                    url=url,
                    status_code=exc.code,
                )
            )
            raise OpenAICompatibleError(f"openai-compatible api returned {exc.code}: {body}") from exc
        except urllib.error.URLError as exc:
            logger.exception(
                format_log(
                    "openai.network_error",
                    model=self.model,
                    url=url,
                    reason=exc.reason,
                )
            )
            raise OpenAICompatibleError(f"openai-compatible api request failed: {exc.reason}") from exc

        try:
            payload = json.loads(response_bytes.decode("utf-8"))
            content = payload["choices"][0]["message"]["content"]
            usage = payload.get("usage", {})
            completion_tokens = int(usage.get("completion_tokens", 0))
            logger.info(
                format_log(
                    "openai.chat_response",
                    model=self.model,
                    completion_tokens=completion_tokens,
                )
            )
            return content, completion_tokens
        except (KeyError, IndexError, TypeError, ValueError, json.JSONDecodeError) as exc:
            logger.exception(
                format_log(
                    "openai.invalid_response",
                    model=self.model,
                    url=url,
                )
            )
            raise OpenAICompatibleError("invalid openai-compatible response shape") from exc


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
