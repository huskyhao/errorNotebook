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
* `LLM_BACKEND=auto|mock|openai|openai_compatible`
* `LLM_MOCK_DELAY_SECONDS=0.2`
* `OPENAI_BASE_URL`
* `OPENAI_API_KEY`
* `OPENAI_MODEL`
* `OPENAI_TIMEOUT_SECONDS`

说明：

* `OCR_BACKEND=auto` 时，会优先尝试 `PaddleOCR`，失败则自动回落到 `mock`
* `LLM_BACKEND=mock` 明确使用可追溯 mock；`LLM_BACKEND=openai` 或 `openai_compatible` 时配置缺失会返回 `AI_CONFIG_MISSING`，不会伪装成成功
* `LLM_BACKEND=auto` 且三项配置完整时才会走真实 LLM；真实调用的超时、429、临时 5xx 会在动作预算内重试
* 如果解析结果出现“当前为 mock 解析结果”，先检查 `GET /internal/v1/health` 的 `llmBackend`；`mock` 表示当前进程明确启用了 mock，需要把本地 `.env` 改为 `LLM_BACKEND=auto`（并重启 AI 服务）后再验证
* 图片解析会优先调用视觉 provider；视觉 provider 暂时不可用时，若文本 provider 可用，会基于 OCR 结果降级解析并返回 `multimodal_fallback_to_ocr` warning，不会静默生成 mock 结果
* `AI_MAX_ATTEMPTS` 默认 3（包含首次调用），结构修复最多 1 次；`AI_ACTION_TIMEOUT_SECONDS` 控制单动作超时

## 启动方式

```powershell
cd ai-service
python -m venv .venv
.venv\Scripts\Activate.ps1
pip install -r requirements.txt
uvicorn app.main:app --reload --host 0.0.0.0 --port 8001
```

运行日志同时输出到启动终端和 `app/.log/<UTC 启动时间>.log`。解析日志只记录 trace、题目 ID、状态、耗时、token 和内容长度；不记录 API Key、完整题干或模型答案。结构或答案完整性校验失败时可检索 `analysis.validation_retry` 与 `analysis.validation_failed`。

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

## 八类题型 Prompt 与响应约定

OCR 结构化会在证据足够时识别 `single_choice`、`multiple_choice`、`true_false`、`fill_blank`、`subjective`、`short_answer`、`essay`、`calculation`；无法确认时保留人工校准状态。解析、追问和 Agent 都会注入题型专用约定，不把所有题型套成单选题：

* 单选：`answer` 为一个选项 key。
* 多选：`answer` 为逗号分隔的多个选项 key，`optionAnalysis` 逐项解释。
* 判断：`answer` 为 `true`/`false`。
* 填空：`answer` 按空位顺序分号分隔，`blankAnswers` 保存逐空答案。
* 主观、简答、论述、计算：`answer` 为参考结论，`steps`、`scoringPoints`、`rubric` 保存解题或评分依据；不直接宣称最终判分。

统一解析响应仍包含 `summary`、`knowledgePoints`、`steps`、`optionAnalysis`、`pitfalls` 和 `reviewAdvice`。没有选项的题型 `optionAnalysis` 应为空；Go 负责保存和展示，Python 不写业务库。

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

`POST /internal/v1/agent/actions` 只接受 Go 构建的题目上下文，Python 不凭 `questionId` 查询业务库。`action` 允许：`diagnose_mistake`、`explain_alternative`、`hint`、`suggest_taxonomy`、`generate_similar_question`、`grade_subjective_answer`。

响应统一包含 `traceId`、`questionId`、`action`、`status`、类型化 `result`、`warnings`、`error` 和 `meta`。`status` 为 `completed`、`needs_input`、`needs_review` 或 `failed`；`meta.source` 明确标记 `mock`/`real`。错误至少包含 `code`、`message`、`retryable`、`traceId`，不透传密钥或上游原始响应。

四类动作结果：`diagnose_mistake` 返回错因、证据、薄弱标签和复习建议；`explain_alternative` 返回换种讲法、重点和检查问题；`hint` 返回 1/2/3 级提示、下一问及泄露标记；`suggest_taxonomy.categoryName` 只能从 Go 提供的大类学科候选中选择，`tagNames` 返回最多 3 个细知识点并可提出新标签，是否创建/应用由 Go 业务端决定。缺少作答返回 `needs_input`，只有错误选项而无过程时只能给可能错因。

P1 结果同样是建议：`generate_similar_question` 返回一个带 `proposalId`、源题指纹、题型、选项、答案、解析和 `qualityStatus` 的候选；`grade_subjective_answer` 只接受主观题和可追溯 rubric/标准答案/解析，返回分项得分、证据、缺失要点、不确定项和人工复核标记。Python 不创建 Question、不修改练习或学习状态。

图片追问的 multipart 契约为：`payload` 是旧 JSON `ChatRequest`，`files` 可重复且最多 4 个，每个文件必须是非空的 `image/png|jpeg|gif|webp|bmp`，Python 使用实际字节作为视觉输入；旧文本请求仍使用 `application/json`。路径、文件名和图片中的指令都不是系统规则。

离线评测案例位于 `evals/p0_cases.json` 和 `evals/p1_cases.json`，P1 共 24 例，覆盖 taxonomy、图片追问、相似题和主观题批改。运行：

```powershell
python evals/run_p0_eval.py
python evals/run_p1_eval.py
python -m pytest -q
```

全量单元测试也可以使用仓库提供的离线 runner；它只在未显式设置时默认使用 mock provider，避免本地 `.env` 导致测试访问真实服务：

```powershell
python run_tests.py
```

未配置真实 provider 时，报告只代表 mock/契约结果，不代表真实模型内容质量。

这样可以先确认 AI 服务本身，再进入 Go 编排联调。

## 与单实例 Go 后端和异步任务的边界

ai-service 不接收或保存浏览器 Cookie、Session Token、用户表和业务数据库；Go 只把当前题目的结构化上下文、学习证据和图片字节传入。Python 返回 OCR/分析的结构化结果、warning 和统一错误，任务状态、重试、租约与最终入库仍由 Go 控制。

图片导入由 Go 异步编排：前端轮询 Go 的 `/api/v1/jobs/{jobId}` 和 `/api/v1/questions/{id}`，不会轮询本服务。OCR 题干不可用时由 Go 暂停，题干可用但置信度不足时可以继续分析并保留 warning。未配置真实 provider 时，结果只代表 mock/契约测试。

OCR 结构化会确定性移除题干前缀中的考试界面噪声，例如 `倒计时00:17:35 24/31单选题（分值3.0分，难度：易）`，清洗后的 `stem` 只保留题目正文；`rawText` 始终保留原始 OCR 内容用于回看，`warnings` 增加 `ocr_exam_ui_noise_removed`。LLM 二次结构校验后还会再次执行同一规则，避免模型把界面信息写回题干。
