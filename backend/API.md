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

响应：

```json
{
  "data": {
    "jobId": "ocr_1748260000000000000",
    "questionId": 1,
    "status": "completed"
  }
}
```

说明：

* 图片导入时，Go 创建 `question + job` 后调用 `ai-service` OCR。
* OCR 成功后回填题干、题型、选项、建议答案、原始 OCR 文本、图片路径等字段。
* 手动导入会直接创建题目并将 OCR 状态置为 `completed`。
* 单题导入成功后会触发解析任务，解析在后台 goroutine 中调用 Python 服务。

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
        "status": "pending"
      }
    ]
  }
}
```

说明：

* Go 会先创建批次、题目占位记录和批次明细。
* 每个文件进入 OCR goroutine，受 `MAX_CONCURRENT_OCR` 并发限制。
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

批量明细状态：`pending` / `processing` / `completed` / `failed` / `needs_review`。

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
* 尝试删除本地上传文件目录。

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

### 3.2 生成解析

`POST /api/v1/questions/{id}/analyze`

行为：

* Go 创建 `analyze` job。
* Go 将题目、选项、用户答案等上下文组装为结构化请求。
* Go 在后台 goroutine 中调用 `ai-service` 的 `/internal/v1/analyze/question`。
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
  "name": "数学",
  "parentId": null
}
```

响应：创建后的分类。

### 6.4 更新分类

`PUT /api/v1/categories/{id}`

请求：

```json
{
  "name": "高中数学",
  "parentId": null
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

响应：当前固定业务用户的会话列表。

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
* `startedAt`
* `finishedAt`
* `createdAt`
* `updatedAt`

任务状态：`pending` / `processing` / `completed` / `failed` / `needs_review`。

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

* 目前尚未接入真实鉴权，业务用户固定为 `1`。
* PDF 导入、试卷拆题和批次校对接口已从 MVP 移除；当前稳定范围是图片导入和手动文本导入。
* 批量导入已经异步处理 OCR 和解析，但查询粒度仍以批次明细状态为主。
* 分类树当前返回扁平节点数组，不在后端递归嵌套。
* 做题会话已支持多题型；学习状态、错题重做策略与推荐练习属于下一阶段。
