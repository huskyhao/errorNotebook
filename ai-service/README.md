# ai-service

`ai-service` 是 ErroNotebook 的 AI 能力服务，当前负责四类能力：

1. OCR 识图与题目结构化
2. 题目解析与 LLM 结构化输出
3. 单题辅导 Agent：错因诊断、换种讲法、分级提示、分类标签建议

它可以单独启动和调试，不需要先启动完整项目。`Go` 业务端联调只是后续集成阶段需要。

## 当前可单独验证的能力

启动服务后，可以直接访问以下接口：

* `GET /internal/v1/health`
* `POST /internal/v1/ocr/parse`
* `POST /internal/v1/analyze/question`
* `POST /internal/v1/chat/question`
* `POST /internal/v1/agent/actions`
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
* `LLM_BACKEND=mock` 明确使用可追溯 mock；`LLM_BACKEND=openai` 或 `openai_compatible` 时配置缺失会返回 `AI_CONFIG_MISSING`，不会伪装成成功
* `LLM_BACKEND=auto` 且三项配置完整时才会走真实 LLM；真实调用的超时、429、临时 5xx 会在动作预算内重试
* `AI_MAX_ATTEMPTS` 默认 3（包含首次调用），结构修复最多 1 次；`AI_ACTION_TIMEOUT_SECONDS` 控制单动作超时

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

正式接口是 multipart：

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
  -F "payload=<analysis-request.json" ^
  -F "file=@test.png;type=image/png"
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

## 单题辅导 Agent

`POST /internal/v1/agent/actions` 只接受 Go 构建的题目上下文，Python 不凭 `questionId` 查询业务库。`action` 仅允许：`diagnose_mistake`、`explain_alternative`、`hint`、`suggest_taxonomy`。

响应统一包含 `traceId`、`questionId`、`action`、`status`、类型化 `result`、`warnings`、`error` 和 `meta`。`status` 为 `completed`、`needs_input`、`needs_review` 或 `failed`；`meta.source` 明确标记 `mock`/`real`。错误至少包含 `code`、`message`、`retryable`、`traceId`，不透传密钥或上游原始响应。

四类动作结果：`diagnose_mistake` 返回错因、证据、薄弱标签和复习建议；`explain_alternative` 返回换种讲法、重点和检查问题；`hint` 返回 1/2/3 级提示、下一问及泄露标记；`suggest_taxonomy` 只在 Go 提供的候选中返回建议且不会自动生效。缺少作答返回 `needs_input`，只有错误选项而无过程时只能给可能错因。

离线评测案例位于 `evals/p0_cases.json`，共 20 例，覆盖 408 四门学科、八类题型、坏输入和故障场景。运行：

```powershell
python evals/run_p0_eval.py
python -m pytest -q
```

未配置真实 provider 时，报告只代表 mock/契约结果，不代表真实模型内容质量。

这样可以先确认 AI 服务本身，再进入 Go 编排联调。
