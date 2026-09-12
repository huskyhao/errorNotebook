# ErroNotebook Backend

Go + Gin + GORM REST API server — the sole business entry point for the ErroNotebook platform.

## Architecture

```
[React SPA :3000] --> [Go+Gin backend :8080] --> [Python FastAPI ai-service :8001]
                            |
                         MySQL
```

The backend handles all business logic, persists domain entities (questions, analyses, jobs, chat, taxonomy), and calls the Python ai-service internally for OCR and LLM analysis. The frontend never calls ai-service directly.

## Tech stack

- **Go 1.26** — language
- **Gin** — HTTP framework
- **GORM** — ORM with MySQL driver
- **MySQL** — primary data store

## Project structure

```
backend/
├── cmd/server/main.go              # entry point
├── internal/
│   ├── config/config.go            # env var loading (.env support)
│   ├── database/
│   │   ├── mysql.go                # MySQL connection
│   │   └── migrate.go             # GORM auto-migration
│   ├── handlers/
│   │   ├── router.go              # Gin router, CORS, route registration
│   │   ├── question.go            # question CRUD + import + answer handlers
│   │   ├── question_ai.go         # internal AI passthrough endpoints
│   │   ├── taxonomy.go            # category & tag handlers
│   │   └── health.go              # health check
│   ├── services/
│   │   ├── question_service.go    # question import/OCR/CRUD logic
│   │   ├── question_ai_service.go # AI passthrough orchestration
│   │   ├── job_service.go         # job lifecycle
│   │   └── taxonomy_service.go    # category & tag logic
│   ├── repository/
│   │   ├── question_repository.go
│   │   ├── analysis_repository.go
│   │   ├── chat_repository.go
│   │   ├── job_repository.go
│   │   └── taxonomy_repository.go
│   ├── models/
│   │   ├── question.go            # Question, QuestionOption, QuestionAsset
│   │   ├── analysis.go            # Analysis
│   │   ├── taxonomy.go            # Category, Tag, QuestionTag
│   │   └── doc.go                 # Job, ChatMessage
│   └── integrations/ai/client.go  # HTTP client for ai-service
└── pkg/response/response.go       # unified JSON response helpers
```

## Prerequisites

- Go 1.26+
- MySQL 8.0+ (a running instance with an empty database)

## Quick start

### 1. Clone and navigate

```bash
cd backend
```

### 2. Configure environment

Copy the example env file and edit it:

```bash
cp .env.example .env
```

Edit `.env` with your settings:

| Variable | Required | Default | Description |
|---|---|---|---|
| `PORT` | No | `8080` | Server listen port |
| `MYSQL_DSN` | **Yes** | — | MySQL connection string |
| `AI_SERVICE_BASE_URL` | **Yes** | — | Python ai-service base URL |
| `AI_SERVICE_TIMEOUT_SECONDS` | No | `15` | Timeout for ai-service calls |

Example `.env`:

```
PORT=8080
MYSQL_DSN=root:password@tcp(127.0.0.1:3306)/erro_notebook?charset=utf8mb4&parseTime=True&loc=Local
AI_SERVICE_BASE_URL=http://127.0.0.1:8001
AI_SERVICE_TIMEOUT_SECONDS=15
```

The config loader checks `.env` first in CWD, then in `backend/`. Existing environment variables take precedence over `.env` values.

### 3. Create the database

```sql
CREATE DATABASE erro_notebook CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

Tables are auto-created by GORM on startup — no manual migrations needed. This is a single-instance application: all questions, categories, tags and learning data belong to the local deployment.

### 4. Install dependencies

```bash
go mod download
```

### 5. Run

```bash
go run ./cmd/server/
```

Or build and run:

```bash
go build -o server ./cmd/server/
./server
```

The server starts with a log line like:

```
backend server listening on :8080
```

Make sure the Python ai-service is also running (on the URL configured in `AI_SERVICE_BASE_URL`).

## API overview

All endpoints under `/api/v1`. For full details see [API.md](./API.md).

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/api/v1/questions/import` | Upload an image/manual question and enqueue OCR |
| `GET` | `/api/v1/questions` | List questions (with filters) |
| `GET` | `/api/v1/questions/:id` | Get question detail |
| `PATCH` | `/api/v1/questions/:id` | Update question fields |
| `DELETE` | `/api/v1/questions/:id` | Delete question and all related data |
| `POST` | `/api/v1/questions/:id/answer` | Submit user answer |
| `POST` | `/api/v1/questions/:id/analyze` | Trigger AI analysis |
| `GET` | `/api/v1/questions/:id/analysis` | Get latest analysis |
| `POST` | `/api/v1/questions/:id/chat` | Send a question-scoped chat message and optional image attachments |
| `GET` | `/api/v1/jobs/:jobId` | Get job status |
| `POST` | `/api/v1/jobs/:jobId/retry` | Retry a terminal failed job |
| `GET` | `/api/v1/categories` | List categories |
| `POST` | `/api/v1/categories` | Create category |
| `GET` | `/api/v1/tags` | List tags |
| `POST` | `/api/v1/tags` | Create tag |

Internal passthrough (under `/api/v1/internal/ai/`):

| Method | Path | Description |
|---|---|---|
| `POST` | `/internal/ai/ocr/parse` | OCR parse relay to ai-service |
| `POST` | `/internal/ai/analyze/question` | Analysis relay to ai-service |

## Response format

All responses use a unified JSON envelope:

```json
{"data": {...}}
{"error": "message"}
```

## Tests

```bash
go test ./...
```

The service and repository layers include unit tests; run the command above before delivery.

## Single-instance data model

The backend does not create users, accounts, login state, cookies or browser sessions. The frontend calls the Go API directly, and all data in the configured MySQL database belongs to this deployment. Categories and tags are ordinary instance-wide vocabulary and can be edited from the workbench.

## Current limitations

- Uploaded images use the durable local object-storage adapter by default; an S3/MinIO adapter can be added behind the same interface
- OCR and analysis run through database-backed workers with automatic retry and lease recovery; a distributed queue is not used yet
- No account, registration, login, OAuth or cross-device identity flow; deploy a separate instance when data isolation is required
- Chat messages are persisted per question and forwarded to the Python AI service; an explicit fallback reply is returned when the AI service is unavailable
- PDF import and paper-splitting APIs are intentionally excluded from the current MVP
- The local object-storage adapter is scoped by `questions/{questionId}`; S3/MinIO remains a future adapter
