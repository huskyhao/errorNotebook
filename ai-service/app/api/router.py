from fastapi import APIRouter

from app.api.routes.analysis import router as analysis_router
from app.api.routes.agent import router as agent_router
from app.api.routes.chat import router as chat_router
from app.api.routes.debug import router as debug_router
from app.api.routes.health import router as health_router
from app.api.routes.ocr import router as ocr_router

api_router = APIRouter(prefix="/internal/v1")
api_router.include_router(health_router, tags=["health"])
api_router.include_router(ocr_router, tags=["ocr"])
api_router.include_router(analysis_router, tags=["analysis"])
api_router.include_router(agent_router, tags=["agent"])
api_router.include_router(chat_router, tags=["chat"])
api_router.include_router(debug_router, tags=["debug"])
