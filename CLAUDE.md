# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working in this repository.

## Project overview

ErroNotebook is an AI-powered error-question workbench for CS/STEM exam prep. Users import questions (screenshot, photo, text), the system OCRs and structures them, users answer, AI generates analysis, and users can follow up with chat — all centered on individual questions.

## Architecture

```
[React SPA :3000] --> [Go+Gin backend :8080] --> [Python FastAPI ai-service :8001]
                            |                           |
                         MySQL                     PaddleOCR / Vision LLM (Qwen3-VL) / Text LLM
```

- **Go backend** is the sole business entry point. Frontend never calls ai-service directly.
- **Python ai-service** is an internal microservice: OCR parsing, diagram vision fallback, LLM analysis. Returns structured JSON only; does not write to business DB.
- Data model centers on `Question` — OCR results, diagram description, analyses, answers, and chat messages all hang off it.

## Service commands

### Backend (`backend/`)

```bash
cd backend
go build ./cmd/server/          # build
go run ./cmd/server/            # run (needs MYSQL_DSN, AI_SERVICE_BASE_URL)
go test ./...                   # unit and contract tests
go vet ./...                    # static checks
```

Required env vars: `MYSQL_DSN`, `AI_SERVICE_BASE_URL`. Optional: `PORT` (default 8080), `AI_SERVICE_TIMEOUT_SECONDS` (default 15). Supports `.env` files in CWD or `backend/`.

### AI service (`ai-service/`)

```bash
cd ai-service
python -m venv .venv && source .venv/bin/activate  # or .venv\Scripts\Activate.ps1 on Windows
pip install -r requirements.txt
uvicorn app.main:app --reload --host 0.0.0.0 --port 8001
python run_tests.py            # offline unit/contract tests
```

Optional: `pip install paddleocr` for real OCR (defaults to mock backend). Swagger docs at `http://localhost:8001/docs`.

Key env vars: `OPENAI_BASE_URL`, `OPENAI_API_KEY`, `OPENAI_MODEL` (set to enable real LLM; defaults to mock), `OCR_BACKEND` (auto/mock/paddleocr). For diagram questions: `VISION_BASE_URL`, `VISION_API_KEY`, `VISION_MODEL` (set to enable vision-LLM fallback when OCR detects diagrams; defaults to Qwen3-VL-32B-Instruct).

### Frontend (`frontend/`)

```bash
cd frontend
npm install
npm start                       # dev server on :3000
npm test -- --watchAll=false   # Jest
npx tsc --noEmit               # type check
npx eslint src --ext .ts,.tsx  # lint
npm run build                  # production build to build/
```

Env: `REACT_APP_API_BASE_URL` (defaults to `http://localhost:8080/api/v1`).

## Key code structure

### Go backend (`backend/internal/`)
- `handlers/` — Gin HTTP handlers (router, question, taxonomy, health, internal AI routes)
- `services/` — business logic (question import/OCR flow, analysis flow, chat, taxonomy)
- `repository/` — GORM data access (question, analysis, chat, job, taxonomy repos)
- `models/` — domain entities (Question, Analysis, Job, ChatMessage, Category, Tag)
- `integrations/ai/` — HTTP client calling ai-service
- `config/` — env var loading with `.env` support
- `pkg/response/` — unified JSON response helpers

### Python ai-service (`ai-service/app/`)
- `api/routes/` — health, ocr, analysis, debug endpoints
- `services/` — OCR service, analysis service, OpenAI (text) client, vision client (multimodal), question structurer
- `services/ocr_backends.py` — `OCRBackend` protocol + Mock + PaddleOCR + factory
- `services/vision_service.py` — `VisionClient` for calling vision LLM (Qwen3-VL) to describe diagrams
- `schemas/` — Pydantic request/response models
- `core/config.py` — `Settings` dataclass from env vars

### Frontend (`frontend/src/`)
- `pages/` — page-level entries for workbench, practice, stats, archive and settings
- `components/` — reusable workbench, taxonomy, chat and dialog components
- `App.css` — shared visual system and workbench layout styles

## Conventions

- **AGENTS.md** defines product direction, service boundaries, and agent working principles — read it before making architectural changes.
- **talk.md** is the collaboration log. Every conversation must be written to talk.md at the end of each session — prepend new entries at the top, don't append. This is a hard requirement, not a suggestion.
- **project.md** defines what to build; **webdesign.md** defines how pages should look and behave.
- The Python service uses a protocol/implementation split for backends (OCR, LLM) — follow this pattern when adding new capabilities.
- API prefix: `/api/v1` (external, Go) and `/internal/v1` (internal, Python).
- All responses use a unified JSON envelope: `{"data": ...}` or `{"error": ...}`.

## Current state

The core loop uses database-backed jobs for import → OCR → analysis, then supports answer submission and question-scoped chat. Anonymous signed sessions scope questions, jobs, analyses, attachments, proposals, practice sessions and learning state. The Python service supports mock and OpenAI-compatible text/vision providers, while Go remains the only business API entry point. Go, Python and frontend tests are present; the Python offline runner and P0/P1 evaluation scripts do not require API credentials.
