from __future__ import annotations

import json

from fastapi import APIRouter, HTTPException, Request, UploadFile

from app.core.config import settings
from app.schemas.chat import ChatImage, ChatRequest, ChatResponse
from app.services.chat_service import ChatService
from app.services.openai_client import OpenAICompatibleError

router = APIRouter()
chat_service = ChatService()


@router.post("/chat/question", response_model=ChatResponse)
async def chat_question(request: Request) -> ChatResponse:
    content_type = request.headers.get("content-type", "")
    if content_type.startswith("multipart/form-data"):
        form = await request.form()
        raw_payload = form.get("payload")
        if not isinstance(raw_payload, str):
            # The legacy multipart shape used by the web client only carried
            # message/files. Keep it useful while requiring the Go payload for
            # the complete question context.
            raise HTTPException(status_code=422, detail={"code": "INVALID_MULTIPART", "message": "payload JSON is required"})
        try:
            payload = ChatRequest.model_validate(json.loads(raw_payload))
        except (json.JSONDecodeError, ValueError) as exc:
            raise HTTPException(status_code=422, detail={"code": "INVALID_PAYLOAD", "message": "payload is not valid chat JSON"}) from exc
        uploads = form.getlist("files") or form.getlist("attachments")
        if len(uploads) > settings.ai_max_image_count:
            raise HTTPException(status_code=413, detail={"code": "TOO_MANY_IMAGES", "message": "at most 4 images are allowed"})
        images: list[ChatImage] = []
        allowed = {"image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp"}
        for upload in uploads:
            if not isinstance(upload, UploadFile):
                raise HTTPException(status_code=400, detail={"code": "INVALID_IMAGE", "message": "invalid image upload"})
            media_type = (upload.content_type or "").lower()
            if media_type not in allowed:
                raise HTTPException(status_code=415, detail={"code": "UNSUPPORTED_IMAGE_TYPE", "message": "only supported image types are allowed"})
            content = await upload.read()
            if not content:
                raise HTTPException(status_code=400, detail={"code": "EMPTY_IMAGE", "message": "image is empty"})
            detected = _detect_image_type(content)
            if detected is None or detected != media_type:
                raise HTTPException(status_code=415, detail={"code": "IMAGE_CONTENT_MISMATCH", "message": "declared MIME does not match image bytes"})
            if len(content) > settings.ai_max_image_bytes:
                raise HTTPException(status_code=413, detail={"code": "IMAGE_TOO_LARGE", "message": "image exceeds 8 MiB"})
            images.append(ChatImage(fileName=upload.filename or "image", contentType=media_type, content=content))
        payload.imageAttachments = images
    else:
        try:
            payload = ChatRequest.model_validate(await request.json())
        except ValueError as exc:
            raise HTTPException(status_code=422, detail={"code": "INVALID_PAYLOAD", "message": "request is not valid chat JSON"}) from exc
    try:
        return await chat_service.chat(payload)
    except OpenAICompatibleError as exc:
        raise HTTPException(
            status_code=exc.status_code,
            detail={"code": exc.code, "message": exc.message, "retryable": exc.retryable, "traceId": payload.traceId},
        ) from exc


def _detect_image_type(content: bytes) -> str | None:
    if content.startswith(b"\x89PNG\r\n\x1a\n"):
        return "image/png"
    if content.startswith(b"\xff\xd8\xff"):
        return "image/jpeg"
    if content.startswith((b"GIF87a", b"GIF89a")):
        return "image/gif"
    if len(content) >= 12 and content[:4] == b"RIFF" and content[8:12] == b"WEBP":
        return "image/webp"
    if content.startswith(b"BM"):
        return "image/bmp"
    return None
