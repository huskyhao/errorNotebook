from __future__ import annotations

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from app.agents.dispatcher import AgentDispatcher
from app.schemas.agent import AgentActionRequest, AgentActionResponse

router = APIRouter()
agent_dispatcher = AgentDispatcher()


@router.post("/agent/actions", response_model=AgentActionResponse)
async def agent_action(payload: AgentActionRequest) -> AgentActionResponse:
    response = await agent_dispatcher.run(payload)
    if response.status == "failed":
        retryable = bool(response.error and response.error.retryable)
        return JSONResponse(status_code=503 if retryable else 502, content=response.model_dump())
    return response
