from __future__ import annotations

import logging
import time
import uuid

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from app.api.router import api_router
from app.core.config import settings
from app.core.logging import format_log, setup_logging


setup_logging()
logger = logging.getLogger("app.main")


def _log_startup_config() -> None:
    logger.info(
        "startup.config | ocr_backend=%s | openai_enabled=%s model=%s base_url=%s | vision_enabled=%s model=%s base_url=%s",
        settings.ocr_backend,
        settings.openai_enabled,
        settings.openai_model or "<unset>",
        settings.openai_base_url or "<unset>",
        settings.vision_enabled,
        settings.vision_model or "<unset>",
        settings.vision_base_url or "<unset>",
    )


def create_app() -> FastAPI:
    _log_startup_config()
    app = FastAPI(
        title=settings.app_name,
        version=settings.app_version,
        docs_url="/docs",
        redoc_url="/redoc",
    )

    @app.middleware("http")
    async def request_logging_middleware(request: Request, call_next):
        trace_id = request.headers.get("x-trace-id") or uuid.uuid4().hex
        request.state.trace_id = trace_id
        started_at = time.perf_counter()

        logger.info(
            format_log(
                "request.started",
                trace_id=trace_id,
                method=request.method,
                path=request.url.path,
                query=str(request.url.query),
            )
        )

        try:
            response = await call_next(request)
        except Exception:
            elapsed_ms = int((time.perf_counter() - started_at) * 1000)
            logger.exception(
                format_log(
                    "request.failed",
                    trace_id=trace_id,
                    method=request.method,
                    path=request.url.path,
                    duration_ms=elapsed_ms,
                )
            )
            return JSONResponse(
                status_code=500,
                content={
                    "detail": "internal server error",
                    "traceId": trace_id,
                },
            )

        elapsed_ms = int((time.perf_counter() - started_at) * 1000)
        response.headers["X-Trace-Id"] = trace_id
        logger.info(
            format_log(
                "request.completed",
                trace_id=trace_id,
                method=request.method,
                path=request.url.path,
                status_code=response.status_code,
                duration_ms=elapsed_ms,
            )
        )
        return response

    app.include_router(api_router)
    return app


app = create_app()
