from __future__ import annotations

from dataclasses import dataclass


@dataclass
class AIServiceError(RuntimeError):
    code: str
    message: str
    retryable: bool = False
    status_code: int = 502

    def __str__(self) -> str:
        return self.message


def classify_provider_error(exc: Exception) -> AIServiceError:
    if isinstance(exc, AIServiceError):
        return exc
    message = str(exc).lower()
    if "401" in message or "403" in message:
        return AIServiceError("PROVIDER_AUTH", "AI provider authentication failed", False, 502)
    if "429" in message or "rate" in message or "limit" in message:
        return AIServiceError("PROVIDER_RATE_LIMIT", "AI provider rate limited the request", True, 503)
    if "timeout" in message or "timed out" in message:
        return AIServiceError("PROVIDER_TIMEOUT", "AI provider request timed out", True, 504)
    if "invalid" in message or "json" in message:
        return AIServiceError("INVALID_OUTPUT", "AI provider returned invalid structured output", True, 502)
    return AIServiceError("PROVIDER_UNAVAILABLE", "AI provider is unavailable", True, 503)
