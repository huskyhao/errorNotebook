from __future__ import annotations

from fastapi import APIRouter, HTTPException, status

from app.schemas.chat import ChatRequest, ChatResponse
from app.services.chat_service import ChatService
from app.services.openai_client import OpenAICompatibleError

router = APIRouter()
chat_service = ChatService()


@router.post("/chat/question", response_model=ChatResponse)
async def chat_question(payload: ChatRequest) -> ChatResponse:
    try:
        return await chat_service.chat(payload)
    except OpenAICompatibleError as exc:
        raise HTTPException(
            status_code=exc.status_code,
            detail={"code": exc.code, "message": exc.message, "retryable": exc.retryable, "traceId": payload.traceId},
        ) from exc
