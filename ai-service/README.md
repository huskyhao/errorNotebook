# ai-service

`ai-service` 是 ErroNotebook 的 AI 能力服务，当前负责两类能力：

1. OCR 识图与题目结构化
2. 题目解析与 LLM 结构化输出

它可以单独启动和调试，不需要先启动完整项目。`Go` 业务端联调只是后续集成阶段需要。

## 当前可单独验证的能力

启动服务后，可以直接访问以下接口：

* `GET /internal/v1/health`
* `POST /internal/v1/ocr/parse`
* `POST /internal/v1/analyze/question`
* `POST /internal/v1/debug/ocr`
* `POST /internal/v1/debug/analyze/mock-question`

其中：

* 正式接口用于后续与 `backend` 对接
* `debug` 接口用于本地快速单测 OCR / LLM，不需要先拼完整业务请求

## 环境变量

参考 [`.env.example`](./.env.example)。

常用项：

* `LOG_LEVEL=INFO|DEBUG|WARNING|ERROR`
* `OCR_BACKEND=auto|mock|paddleocr`
* `OCR_MOCK_DELAY_SECONDS=0.2`
* `LLM_BACKEND=mock`
* `LLM_MOCK_DELAY_SECONDS=0.2`
* `OPENAI_BASE_URL`
* `OPENAI_API_KEY`
* `OPENAI_MODEL`
* `OPENAI_TIMEOUT_SECONDS`

说明：

* `OCR_BACKEND=auto` 时，会优先尝试 `PaddleOCR`，失败则自动回落到 `mock`
* 只要 `OPENAI_BASE_URL / OPENAI_API_KEY / OPENAI_MODEL` 三项配置完整，解析会走真实 LLM
* 如果三项未配置完整，则解析默认走 `mock`

## 启动方式

```powershell
cd ai-service
python -m venv .venv
.venv\Scripts\Activate.ps1
pip install -r requirements.txt
uvicorn app.main:app --reload --host 0.0.0.0 --port 8001
```

启动后访问：

* `http://localhost:8001/docs`
* `http://localhost:8001/internal/v1/health`

## 单独测试 OCR

### 方式一：Swagger

打开 `http://localhost:8001/docs`，直接调用：

* `POST /internal/v1/debug/ocr`

这个接口适合本地快速传图测试。

### 方式二：curl

```powershell
curl.exe -X POST "http://localhost:8001/internal/v1/debug/ocr" ^
  -H "accept: application/json" ^
  -F "question_id=10001" ^
  -F "source_type=image" ^
  -F "file=@test.png;type=image/png"
```

如果你要验证正式接口，也可以调用：

```powershell
curl.exe -X POST "http://localhost:8001/internal/v1/ocr/parse" ^
  -H "accept: application/json" ^
  -F "question_id=10001" ^
  -F "source_type=image" ^
  -F "trace_id=ocr-manual-test-001" ^
  -F "file=@test.png;type=image/png"
```

## 单独测试 LLM 解析

### 快速调试

先用调试接口验证解析链路是否通：

```powershell
curl.exe -X POST "http://localhost:8001/internal/v1/debug/analyze/mock-question" ^
  -H "accept: application/json" ^
  -F "question_id=10001" ^
  -F "trace_id=llm-manual-test-001" ^
  -F "stem=已知函数 f(x)=x^2-4x+3，求其最小值。" ^
  -F "user_answer=1"
```

这个接口不依赖 OCR 结果，适合先验证：

* 服务是否可用
* mock 解析是否可用
* OpenAI 兼容接口是否已正确配置

### 验证正式解析接口

正式接口是：

* `POST /internal/v1/analyze/question`

建议从 OCR 返回的 `structuredQuestion` 复制一份，组一个 JSON 请求体再测。例如：

```json
{
  "traceId": "llm-manual-test-002",
  "questionId": 10001,
  "userAnswer": "A",
  "context": {
    "source": "manual-test"
  },
  "question": {
    "stem": "下列说法正确的是：",
    "questionType": "single_choice",
    "options": [
      { "key": "A", "content": "选项 A" },
      { "key": "B", "content": "选项 B" }
    ],
    "assets": [],
    "suggestedAnswer": "A",
    "rawText": "下列说法正确的是：A. 选项 A B. 选项 B",
    "warnings": [],
    "metadata": {
      "sourceType": "image",
      "hasDiagram": false,
      "ocrConfidence": 0.98,
      "importMode": "single_question",
      "extractionMethod": "mock"
    }
  }
}
```

调用示例：

```powershell
curl.exe -X POST "http://localhost:8001/internal/v1/analyze/question" ^
  -H "accept: application/json" ^
  -H "Content-Type: application/json" ^
  -d "@analysis-request.json"
```

## 日志说明

服务已补充基础请求级日志和服务日志，启动后终端会看到类似信息：

* `request.started`
* `request.completed`
* `request.failed`
* `ocr.started`
* `ocr.completed`
* `analysis.started`
* `analysis.completed`
* `openai.request`
* `openai.response`

日志里会带上这些关键字段：

* `trace_id`
* `question_id`
* `backend`
* `duration_ms`
* `status_code`
* `completion_tokens`

排查时建议优先看同一个 `trace_id` 的整条链路。

## OCR 真实能力说明

如果你想验证真实 OCR，而不是 mock，需要额外安装 `PaddleOCR`：

```powershell
pip install paddleocr
```

然后把环境变量设为：

```powershell
$env:OCR_BACKEND="paddleocr"
```

如果依赖没装全，`auto` 模式会自动回落到 `mock`，服务仍然能启动。

## 建议的本地调试顺序

1. 启动 `uvicorn`
2. 先访问 `/internal/v1/health` 看后端状态
3. 用 `/internal/v1/debug/ocr` 验证 OCR
4. 用 `/internal/v1/debug/analyze/mock-question` 验证解析
5. 再测正式接口 `/internal/v1/ocr/parse` 和 `/internal/v1/analyze/question`

这样可以先确认 AI 服务本身，再进入 Go 编排联调。
