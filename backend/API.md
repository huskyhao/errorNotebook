# Backend API

本文档对齐当前 `backend/` 真实实现，作为第一阶段 MVP 联调入口。

统一前缀：`/api/v1`

统一响应：

```json
{
  "data": {}
}
```

统一错误：

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "error message"
  }
}
```

## 0. 单实例数据模型

Go 不创建或校验用户、账号、登录态、Cookie 或浏览器 session。前端请求无需携带 `credentials`，也不需要提交 `userId`。题目、解析、任务、聊天、附件、proposal、练习、学习状态、分类和标签都直接属于当前部署实例；对象 key 使用 `questions/{questionId}/...`。

数据库启动时会确保 `数据结构`、`计算机组成原理`、`操作系统`、`计算机网络` 四个顶层分类存在。分类和标签是当前实例的普通词表，AI 只能从现有分类中选择，并由 Go 校验后应用。

## 1. 服务边界

### Go `backend`

负责：

* 对前端提供统一 REST API。
* 题目录入、批量导入、题目详情、作答、解析状态、聊天记录、收藏、分类、标签、做题会话等业务能力。
* 持久化 `Question / Job / Analysis / ChatMessage / Category / Tag / BatchImport / PracticeSession` 等业务数据。
* 调用 Python `ai-service`，并维护 OCR、解析、追问相关状态。

不负责：

* OCR 算法本身。
* LLM 解析和追问生成本身。
* 让前端直接访问 Python 服务。

### Python `ai-service`

当前被 Go 调用的内部接口：

* `POST /internal/v1/ocr/parse`
* `POST /internal/v1/analyze/question`
* `POST /internal/v1/chat/question`

Python 服务只返回结构化 JSON，不直接写业务库。

### 1.1 AI Provider 状态

`GET /api/v1/settings/ai` 由 Go 读取 Python `/internal/v1/health` 的脱敏配置状态：

```json
{
  "data": {
    "text": { "provider": "openai_compatible", "model": "模型名", "configured": true },
    "vision": { "provider": "not_configured", "model": null, "configured": false },
    "source": "server_env"
  }
}
```

API Key 只允许通过 AI 服务端 `.env` 或 secrets 注入，永不返回原文、URL 或日志。设置页是只读状态页，不提供浏览器填写、`localStorage` 保存或前端直连 Python。

## 2. 题目接口

### 2.1 单题导入

`POST /api/v1/questions/import`

支持：

* `multipart/form-data` 上传图片文件。
* `rawText + sourceType=manual` 手动文本导入。

请求字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file` | file | 图片导入必填 | 图片文件 |
| `rawText` | string | 手动导入必填 | 原始题目文本 |
| `sourceType` | string | 否 | `image` / `manual`，默认 `image` |

响应（上传接口立即返回占位题目，任务在后台执行）：

```json
{
  "data": {
    "jobId": "ocr_1748260000000000000",
    "questionId": 1,
    "status": "queued"
  }
}
```

说明：

* 图片导入时，Go 先把原图写入对象存储，再创建 `question + job`，不在上传请求中调用 OCR。
* OCR 成功后回填题干、题型、选项、建议答案、原始 OCR 文本、图片路径等字段。
* 结构化阶段会剥离题干开头的倒计时、题目进度、题型、分值、难度和“第 N 题”等考试界面信息，并保留原始 OCR 证据；发生清洗时写入 `ocr_exam_ui_noise_removed` warning。
* 手动导入会直接创建题目并将 OCR 状态置为 `completed`。
* 图片 OCR 成功后会自动创建解析任务，OCR 和解析均由数据库 worker 在后台调用 Python 服务；手动创建题目如需解析，由前端调用 `POST /api/v1/questions/{id}/analyze`。

Go 调 Python 时保证 `question.warnings` 即使为空也序列化为 `[]`，不得发送 `null`；否则 Python 的结构化契约会拒绝该分析请求。

### 2.2 批量导入

`POST /api/v1/questions/batch-import`

请求格式：`multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `files` | file[] | 是 | 多个图片文件 |
| `sourceType` | string | 否 | 默认 `image` |

响应：

```json
{
  "data": {
    "batchId": 1,
    "total": 2,
    "questions": [
      {
        "fileIndex": 0,
        "fileName": "q1.png",
        "questionId": 10,
        "status": "queued"
      }
    ]
  }
}
```

说明：

* Go 会先创建批次、题目占位记录和批次明细。
* 每个文件先保存为独立对象并创建 OCR Job，由 `TASK_WORKER_COUNT` 个数据库 worker 按队列处理。
* 单个文件 OCR 完成后会自动触发解析。

### 2.3 查询批量导入状态

`GET /api/v1/batch-imports/{id}`

响应：

```json
{
  "data": {
    "batchId": 1,
    "total": 2,
    "questions": [
      {
        "fileIndex": 0,
        "fileName": "q1.png",
        "questionId": 10,
        "status": "completed",
        "error": ""
      }
    ]
  }
}
```

批量明细状态：`queued` / `processing` / `completed` / `failed` / `needs_review`。`processingStage` 会区分 `ocr`、`analysis_queued`、`analysis` 和终态。

### 2.4 题目列表

`GET /api/v1/questions`

支持筛选：

| 参数 | 说明 |
| --- | --- |
| `categoryId` | 分类 ID；传 `0` 表示未分类 |
| `isFavorited` | `true` / `false` |
| `tagIds` | 逗号分隔的标签 ID，如 `1,2` |
| `questionType` | 八类题型之一：`single_choice` / `multiple_choice` / `true_false` / `fill_blank` / `subjective` / `short_answer` / `essay` / `calculation` |
| `ocrStatus` | OCR 状态 |
| `analysisStatus` | 解析状态 |
| `keyword` | 按题干模糊搜索 |

响应：`QuestionDetail[]`。

### 2.5 题目详情

`GET /api/v1/questions/{id}`

响应字段包括：

* `id`
* `stem`
* `questionType`
* `correctAnswer`
* `userAnswer`
* `ocrStatus`
* `analysisStatus`
* `sourceType`
* `rawOcrText`
* `categoryId`
* `categoryName`
* `diagramDescription`
* `hasDiagram`
* `imagePath`
* `isFavorited`
* `options`
* `assets`
* `tags`

### 2.6 更新题目

`PATCH /api/v1/questions/{id}`

请求：

```json
{
  "stem": "题干",
  "questionType": "single_choice",
  "correctAnswer": "B",
  "categoryId": 1,
  "options": [
    { "key": "A", "content": "选项 A" },
    { "key": "B", "content": "选项 B" }
  ]
}
```

说明：

* 所有字段均可按需传入。
* `options` 传入时会整体替换该题选项。
* 返回更新后的题目详情。

### 2.7 删除题目

`DELETE /api/v1/questions/{id}`

行为：

* 删除题目。
* 删除关联选项、素材、解析记录、聊天记录、任务记录、题目标签关联。
* 尝试删除题目原图对应的对象存储对象。

成功响应：`204 No Content`

## 3. 作答与解析

### 3.1 提交作答

`POST /api/v1/questions/{id}/answer`

请求：

```json
{
  "userAnswer": "B"
}
```

响应：更新后的题目详情。

分类字段约束：`categoryId` 是题目的唯一学科分类，传正整数只能指向当前用户可见的系统/自有顶层分类，传 `null` 表示未分类；Go 会拒绝不存在、他人私有或带父级的分类。

### 3.2 生成解析

`POST /api/v1/questions/{id}/analyze`

行为：

* Go 创建 `analyze` job。
* Go 将题目、选项、用户答案等上下文组装为结构化请求。
* Go worker 在领取 analyze Job 后调用 `ai-service` 的 `/internal/v1/analyze/question`，支持自动重试和租约恢复。
* 成功后写入 `analyses`，并将 `questions.analysis_status` 更新为 `completed`。
* 失败后将 `questions.analysis_status` 更新为 `failed`，并记录 job 错误。

响应：

```json
{
  "data": {
    "jobId": "analyze_1748260000000000000",
    "questionId": 1,
    "status": "processing"
  }
}
```

### 3.3 获取解析

`GET /api/v1/questions/{id}/analysis`

返回该题最新一条解析记录。

响应：

```json
{
  "data": {
    "id": 1,
    "questionId": 1,
    "provider": "ai-service",
    "answer": "B",
    "content": {
      "summary": "解析摘要",
      "steps": []
    },
    "createdAt": "2026-08-11T00:00:00Z"
  }
}
```

## 4. 题目追问

### 4.1 发送追问

`POST /api/v1/questions/{id}/chat`

请求：

```json
{
  "message": "为什么 B 不对？"
}
```

当前真实链路：

* Go 保存用户消息。
* Go 读取题目、选项、用户答案、最新解析和历史聊天记录。
* Go 调用 Python `ai-service` 的 `/internal/v1/chat/question`。
* Go 保存 assistant 回复。
* 如果 Python 服务暂时不可用，Go 会保存一条本地降级回复，前端接口不变。
* 前端仍只调用 Go 的 `/api/v1/questions/{id}/chat`。

响应：该题完整聊天记录，按 `id asc` 排序。

### 4.2 获取聊天记录

`GET /api/v1/questions/{id}/chat`

响应：该题聊天记录数组。

## 5. 收藏与标签

### 5.1 收藏题目

`POST /api/v1/questions/{id}/favorite`

请求：

```json
{
  "isFavorited": true
}
```

响应：更新后的题目详情。

### 5.2 设置题目标签

`POST /api/v1/questions/{id}/tags`

请求：

```json
{
  "tagIds": [1, 2]
}
```

说明：

* 传入的 `tagIds` 会整体替换该题当前标签关联。
* 传空数组表示清空标签。
* Go 会去重并校验所有 tag ID；不存在的标签返回 `400 INVALID_TAG_IDS`。

响应：更新后的题目详情。

### 5.3 标签列表

`GET /api/v1/tags`

响应：标签数组。

### 5.4 创建标签

`POST /api/v1/tags`

请求：

```json
{
  "name": "函数"
}
```

响应：创建后的标签。

### 5.5 删除标签

`DELETE /api/v1/tags/{id}`

行为：

* 删除标签。
* 删除该标签与题目的关联。

成功响应：`204 No Content`

## 6. 分类

### 6.1 分类列表

`GET /api/v1/categories`

响应：分类数组，按 `id desc` 排序。

### 6.2 分类树

`GET /api/v1/categories/tree`

响应：

```json
{
  "data": {
    "categories": [
      {
        "id": 1,
        "name": "数学",
        "parentId": null,
        "questionCount": 12
      }
    ],
    "uncategorized": 3
  }
}
```

说明：

* 当前返回的是带 `parentId` 和 `questionCount` 的扁平节点数组，由前端组织展示。
* `uncategorized` 表示未分类题目数量。

### 6.3 创建分类

`POST /api/v1/categories`

请求：

```json
{
  "name": "计算机网络"
}
```

分类是每题唯一、稳定的顶层学科维度；当前不创建子分类。`parentId` 仅为旧数据兼容保留，创建/更新时传入非空值会返回 `400 INVALID_CATEGORY`。TCP、UDP 等细粒度知识点应通过标签接口管理。

响应：创建后的分类。

### 6.4 更新分类

`PUT /api/v1/categories/{id}`

请求：

```json
{
  "name": "计算机网络"
}
```

响应：更新后的分类。

### 6.5 删除分类

`DELETE /api/v1/categories/{id}`

行为：

* 删除分类。
* 子分类会提升到被删除分类的父级。
* 属于该分类的题目会被置为未分类。

成功响应：`204 No Content`

### 6.6 AI 自动分类与标签

AI 解析结果可以包含可选的 `content.taxonomySuggestion`：

```json
{
  "categoryName": "计算机网络",
  "tagNames": ["TCP", "拥塞控制"],
  "confidence": 0.86
}
```

Python 只负责生成该字段，不直接写入业务库。`categoryName` 只能命中 Go 给出的顶层学科候选；`tagNames` 最多 3 个，优先复用已有标签，也可提出新的细粒度知识点。Go 校验后把新标签写入当前实例词表，再保存解析记录，并在分析完成时自动应用到尚未人工设置 taxonomy 的题目。没有匹配的大类学科时保持未分类，用户可在题目详情手动修改。

## 7. 做题会话

### 7.1 创建做题会话

`POST /api/v1/practice-sessions`

请求：

```json
{
  "name": "今日混合练习",
  "questionIds": [1, 2, 3]
}
```

说明：

* `questionIds` 必填且不能为空。
* 支持八类题型：`single_choice`、`multiple_choice`、`true_false`、`fill_blank`、`subjective`、`short_answer`、`essay`、`calculation`。
* 单选、多选、判断和填空题可基础自动判分；主观、简答、论述和计算题提交后进入 `manual_required`。
* 若不传 `name`，后端会生成默认名称。

响应：做题会话详情。

### 7.2 做题会话列表

`GET /api/v1/practice-sessions`

响应：当前实例中的练习会话列表。

### 7.3 做题会话详情

`GET /api/v1/practice-sessions/{id}`

响应字段包括：

* `id`
* `name`
* `status`
* `totalCount`
* `correctCount`
* `createdAt`
* `questions`

会话状态：`in_progress` / `submitted`。

题目状态：`unanswered` / `answered` / `skipped`。

### 7.4 作答会话内题目

`POST /api/v1/practice-sessions/{id}/answer`

请求：

```json
{
  "orderIndex": 0,
  "userAnswer": "A"
}
```

响应：

```json
{
  "data": {
    "status": "answered"
  }
}
```

### 7.5 跳过会话内题目

`POST /api/v1/practice-sessions/{id}/skip`

请求：

```json
{
  "orderIndex": 0
}
```

响应：

```json
{
  "data": {
    "status": "skipped"
  }
}
```

### 7.6 提交做题会话

`POST /api/v1/practice-sessions/{id}/submit`

行为：

* 计算每题 `isCorrect`。
* 将会话状态更新为 `submitted`。
* 返回结果详情。

响应：做题结果。

### 7.7 获取做题结果

`GET /api/v1/practice-sessions/{id}/results`

说明：

* 仅已提交会话可获取结果。
* 未提交会话会返回错误。

响应字段包括：

* `id`
* `name`
* `status`
* `totalCount`
* `correctCount`
* `scorePercent`
* `createdAt`
* `questions`

## 8. 任务状态

`GET /api/v1/jobs/{jobId}`

返回字段：

* `id`
* `jobId`
* `questionId`
* `jobType`
* `status`
* `errorCode`
* `errorMessage`
* `attempts`
* `maxAttempts`
* `processingStage`
* `nextRunAt`
* `lockedAt`
* `startedAt`
* `finishedAt`
* `createdAt`
* `updatedAt`

任务状态：`queued` / `processing` / `completed` / `needs_review` / `failed`（旧数据库中的 `pending` 仍可被 worker 兼容接管）。题目自身分别返回 `ocrStatus` 与 `analysisStatus`，分析尚未生成时是 `queued`/`processing`，不是“无解析”。

图片分析优先把 Go 已校验的原图通过 multipart 交给 Python 视觉 provider；视觉 provider 超时或暂时不可达时，Python 若仍有可用文本 provider，会基于 OCR 结构化结果继续解析，并在 AI 响应 warning 中返回 `multimodal_fallback_to_ocr`。文本 provider 也不可用时返回明确的 `PROVIDER_TIMEOUT`/`PROVIDER_UNAVAILABLE`，Go 按 Job 重试策略处理，不回退到 mock 成功结果。

重试失败任务：

* `POST /api/v1/jobs/{jobId}/retry`：重置失败 Job 并重新入队。
* `POST /api/v1/questions/{id}/ocr/retry`：重试该题最近一次失败的 OCR Job。

## 9. 健康检查与内部 AI 代理

健康检查：

* `GET /health`
* `GET /api/v1/health`

内部 AI 代理接口：

* `POST /api/v1/internal/ai/ocr/parse`
* `POST /api/v1/internal/ai/analyze/question`

说明：

* 这两个接口由 Go 转发到 Python AI 服务，主要用于内部调试或后端代理。
* 前端业务页面不应直接依赖这些内部 AI 代理接口。

## 10. 当前限制

## 10. P0 单题辅导 Agent

### 10.1 触发动作

`POST /api/v1/questions/{id}/agent-actions`

请求：

```json
{
  "action": "hint",
  "params": {"hintLevel": 1}
}
```

Go 根据题目、作答、最新解析、必要对话和现有分类/标签候选构建上下文，再调用 Python `/internal/v1/agent/actions`。前端不直连 Python。动作仅允许 `diagnose_mistake`、`explain_alternative`、`hint`、`suggest_taxonomy`；Python 返回类型化结果和 `completed` / `needs_input` / `needs_review` / `failed` 状态。

`diagnose_mistake` 的成功结果包含 `mistakeReason`、`reasonType`、`evidence`、`weaknessTags`、`reviewAdvice`、`uncertainties`。Go 仅在证据充分且题目内容指纹未变化时写入学习状态，不修改掌握度、正确率、错题次数、连续答对和复习日期。AI 失败时保留旧值。

### 10.2 应用 taxonomy 建议

`POST /api/v1/questions/{id}/taxonomy-suggestion/apply`

图片解析完成后，若题目尚未手动设置 taxonomy，Go 会自动读取并校验建议，在事务中应用当前用户可见的顶层分类和标签；用户仍可通过题目更新/标签接口手动调整。该接口保留用于历史数据修复或显式 API 调用，正常前端流程不会要求点击“应用建议”，重复调用幂等；无匹配、候选已删除和越权访问返回明确错误。

Python 侧动作契约、错误结构、multipart 解析示例和离线评测见 `ai-service/README.md` 与 `ai-service/evals/`。当前未配置真实 provider 时，测试结果只代表 mock/契约链路。

## 10.3 P1 AI proposal 与图片追问

`POST /api/v1/questions/{id}/agent-actions` 新增 `generate_similar_question`。请求可带 `params.sourceFingerprint`、`targetDifficulty`、`questionType` 和 `allowedKnowledgePoints`；Go 会把 Python 返回的单题候选持久化为 `AIProposal(status=pending)`。候选只在确认后创建正式 `Question(sourceType=ai_generated)`：

* `GET /api/v1/questions/{id}/ai-proposals/{proposalId}` 查看候选
* `POST /api/v1/questions/{id}/ai-proposals/{proposalId}/confirm` 幂等确认创建
* `POST /api/v1/questions/{id}/ai-proposals/{proposalId}/reject` 放弃候选

确认会重新校验源题指纹、proposal 状态和题型/选项答案契约；过期、拒绝、源题变更均返回 `409 PROPOSAL_NOT_APPLICABLE`，重复确认返回同一 `createdQuestionId`。

`POST /api/v1/questions/{id}/chat` 继续兼容旧 JSON；带图使用 multipart `message` + 重复 `attachments`。Go 先把图片保存到服务端对象存储，再读取已校验的实际字节，以内部 multipart `payload` + `files` 调用 Python；AI 失败只保留用户消息和附件元数据，不写伪成功 assistant 消息。

## 10.4 P1 主观题辅助批改

* `POST /api/v1/practice-sessions/{id}/questions/{orderIndex}/grade-suggestion`：请求 `{ "params": { "maxScore": 10, "rubric": ["观点", "依据"] } }`，返回 Python 类型化评分建议并保存 pending proposal。
* `GET /api/v1/practice-sessions/{id}/questions/{orderIndex}/grade-suggestion/{proposalId}`：查看同一作答版本的评分建议。
* `POST /api/v1/practice-sessions/{id}/questions/{orderIndex}/grade-suggestion/confirm`：请求 `{ "proposalId": "...", "score": 6, "feedback": "..." }`，Go 重新校验作答指纹后应用；重复确认幂等。

评分建议包含 `suggestedScore`、`maxScore`、`criteriaResults`、`strengths`、`missingPoints`、`feedback`、`evidence`、`uncertainties`、`confidence` 和 `requiresHumanReview`。AI 不直接改变最终成绩、掌握度或错题统计；界面使用“AI 建议分数”，最终采用必须人工确认。

## 11. 当前限制

* 当前为单实例模式，不提供注册、登录、账号恢复或多用户隔离。
* PDF 导入、试卷拆题和批次校对接口已从 MVP 移除；当前稳定范围是图片导入和手动文本导入。
* 图片先写入持久化对象存储，再由数据库 worker 异步处理 OCR 和解析；默认适配器为本地对象存储，后续可替换为 S3/MinIO。
* Worker 使用数据库租约、自动退避重试和过期任务接管；暂未引入独立消息队列。
* 分类树当前返回扁平节点数组，不在后端递归嵌套。
* 做题会话已支持多题型；学习状态与五类推荐练习已接入当前 MVP，推荐策略仍是简单规则，后续再迭代。
