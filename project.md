# 项目名称

ErroNotebook

副标题：面向考研 408 与理工科刷题场景的 AI 错题复习工作台

---

# 1. 项目定位

ErroNotebook 的核心目标不是做泛题库、试卷拆题系统或普通聊天机器人，而是做一个以 `Question` 为中心的 AI 错题复习工作台。

当前 MVP 主线为：

1. 单题图片导入或手动创建
2. OCR 结构化与人工编辑
3. 用户作答与基础判分
4. AI 生成解析
5. 围绕当前题继续追问
6. agent 归纳错因与薄弱点
7. 系统追踪掌握度并推荐下一组复习题
8. 用户进入科学练习并持续更新学习状态

当前 P1 单题能力已纳入同一闭环：解析会在现有候选范围内生成 taxonomy，题目尚未人工设置时由 Go 自动应用，用户发现问题后可手动调整；追问图片以实际字节进入 Python 视觉 provider；相似题先保存为待确认 proposal；主观题只生成带评分依据的 AI 建议，最终确认和学习状态仍由 Go 控制。

PDF 上传、试卷拆题、批次校对与整卷作答已经从当前 MVP 页面、接口和实现主线移除。PDF 只作为未来可能重新评估的可选扩展，不预留当前交付承诺。

---

# 2. 目标用户

## 核心用户

* 考研 408 学生
* 计算机及理工科刷题用户
* 需要从截图、拍照题或手动记录中沉淀错题的人

## 典型场景

* 做题后，把错题截图导入系统并记录错误原因
* 遇到一道不会的题，上传后直接获得结构化讲解
* 已经看过解析，但仍不理解某个步骤，继续追问
* 复习时按科目、知识点、掌握状态回看错题

---

# 3. 产品目标

## 第一阶段目标（MVP）

优先完成“单题导入/创建 → 作答 → AI 解析 → 追问 → 错因归纳 → 复习推荐 → 科学练习”的稳定闭环。

MVP 保留八类题型：`single_choice`、`multiple_choice`、`true_false`、`fill_blank`、`subjective`、`short_answer`、`essay`、`calculation`。客观题与可精确匹配的填空题进行基础自动判分；主观题、简答题、论述题和计算题进入待批改状态。

## 第二阶段目标

当前已从“导入/解析可用”推进到“校准/错因/学习状态/推荐练习”闭环。第二阶段的实现基线是：

* `QuestionLearningState` 成为题目学习闭环的核心扩展模型
* 右侧详情区支持题干、题型、答案、选项的人工校准
* OCR/结构化 warning 统一转成轻量 `qualityStatus`
* “归纳错因”从 prompt action 升级为可保存的结构化学习信息
* 掌握度、错题次数、连续答对和下次复习时间随练习结果更新
* 推荐练习基于今日复习、最近错题、薄弱专项、新题巩固、随机混合五组简单规则

## 第三阶段目标

* 个性化练习策略持续优化
* 知识图谱/知识点关联
* 学习数据看板
* 基于历史错题生成专项训练
* 重新评估 PDF 是否值得作为独立可选扩展

---

# 4. 产品边界

## 当前明确要做

* 单题导入与管理
* OCR 结构化识别
* 导入后质量检查与人工校准
* 题目展示与作答承载
* AI 标准解析与问答
* 错因归纳、薄弱标签与复习建议落库
* 学习状态追踪与推荐练习入口
* 分类、标签、状态管理
* 题目详情页与对话工作台

## 当前不优先做

* 大规模公开题库运营
* 社区、分享、评论
* 复杂教务/班级体系
* 在线考试系统
* 实时协作编辑
* PDF 上传、试卷拆题、批次校对与整卷作答

这样收敛范围的原因很直接：先把“单题深度处理体验”做好，才有后续扩展价值。

---

# 5. 核心能力拆解

## 5.1 题目录入

当前支持两类输入：

* 图片上传：截图、拍照、剪贴板图片
* 手动输入：OCR 失败时兜底

导入控件只接受图片文件，不接受 PDF。多图片批量导入仍通过现有批量导入能力完成，但每个结果仍落为独立 `Question`。

多图片批量导入必须按单张图片独立流转。每个 batch item 都应记录 `queued -> processing -> completed / failed` 的终态，并记录当前处理阶段与失败原因。OCR、保存、结构化或分析触发任一步失败，都不能让前端永久停在“导入中 3/4”；成功题目正常进入题库，失败文件在进度或错误提示中体现。

题目录入结果必须统一为结构化对象，而不是只保存原图。

建议最小结构：

```json
{
  "stem": "题干内容",
  "options": [
    { "key": "A", "content": "选项 A" },
    { "key": "B", "content": "选项 B" }
  ],
  "questionType": "single_choice",
  "sourceType": "image",
  "assets": [],
  "rawText": "OCR 原始文本"
}
```

## 5.2 OCR 与结构化解析

系统不只要“识别文字”，还要做题目结构恢复：

* 识别题号
* 识别题干
* 识别选项
* 判断题型
* 识别图片/公式/表格占位
* 尝试识别标准答案区域

注意：

* OCR 输出应区分“原始 OCR 结果”和“清洗后的题目结构”
* 结构化结果应是标准 JSON，而不是纯文本说明
* OCR 后、入库前需要做二次结构化纠错，先用确定性规则剥离题干开头的倒计时、题号进度、题型、分值、难度等考试界面信息，再交给 LLM 做题面结构校验；`rawText` 保留原始证据，发生清洗时记录 `ocr_exam_ui_noise_removed`
* LLM 校验只负责恢复题干与 A/B/C/D 选项边界，不负责解题、不生成答案解析
* 需要清理 `OA.`、`O B.`、`D。`、孤立 `O`、孤立 `.` 等选项噪声
* 如果 B 与 D 之间存在明显选项内容但缺少 `C.` 标记，LLM 校验可恢复为 C；如果确实无法确认，不能编造内容
* 选项不完整时保留 `rawOcrText`，并通过 `structureWarnings`、`structureConfidence`、`parseSource` 提示质量，不把选项残片塞进题干

## 5.3 题目展示与作答

题目页不只是展示 OCR 结果，还需要提供作答承载能力：

* 单选题支持即时选择
* 多选题支持多项勾选
* 判断题支持“正确 / 错误”选择
* 填空题支持文本输入
* 主观题、简答题、论述题、计算题支持多行文本输入
* 用户作答记录应绑定到题目

这样做的价值是：

* 用户可以在工作台内完成“看题 - 思考 - 作答 - 对答案”的闭环
* 后续可扩展“重做记录”和“掌握度判断”

## 5.4 分类与标签

分类与标签只承担不同层级的职责：

* `category`：每道题最多一个，表示稳定、较高层级的学科分类，例如计算机网络、计算机组成原理、操作系统、数据结构。
* `tags`：每道题可有多个，表示细粒度知识点或检索维度，例如 TCP、UDP、HTTP、拥塞控制、408、真题。

`category_id` 保持 Question 上的单值外键，不引入 Question–Category 多对多表。当前 MVP 的分类不创建子分类，`parent_id` 仅为兼容历史数据保留；新建和更新分类必须是顶层学科。TCP、UDP 等知识点不得作为 category，应该作为 tags。

前端详情区明确显示“学科分类（单选）”与“知识点标签（可多选）”。人工可以随时修改；标签接口整体替换该题标签关联，并由 Go 去重、校验 ID 后持久化。分类删除不会删除题目，题目会移动到未分类；`全部题目` 和 `未分类` 属于系统视图。

taxonomy 采用当前实例统一词表：系统预置 `数据结构`、`计算机组成原理`、`操作系统`、`计算机网络` 四个 408 顶层分类，单实例用户可以新建、修改和删除分类/标签。AI 只返回可选的 `taxonomySuggestion`（`categoryName`、`tagNames`、可选 `confidence`）：category 只能从大类学科候选中选择，每题最多 3 个细知识点 tag，可复用已有项或提出新项。Go 将新 tag 写入当前实例词表，校验后在题目分析完成时自动应用；用户不需要确认，发现问题后可在右侧手动调整。本轮不把知识点升级为分类。

## 5.5 AI 解析

AI 解析建议输出为结构化片段，而不是一整段纯文本。最少包含：

* 正确答案
* 核心结论
* 解题步骤
* 选项逐项判断
* 易错点
* 关联知识点
* 复习建议

建议结构：

```json
{
  "answer": "D",
  "summary": "本题考查系统调用执行时的内核态切换与现场保护。",
  "steps": ["...", "..."],
  "optionAnalysis": {
    "A": "...",
    "B": "...",
    "C": "...",
    "D": "..."
  },
  "pitfalls": ["..."],
  "knowledgePoints": ["系统调用", "内核态与用户态"],
  "reviewAdvice": ["建议复习中断与异常的区别"]
}
```

## 5.6 对话追问

对话必须绑定“当前题目上下文”，否则会退化为普通聊天。

建议支持：

* 为什么不是 A？
* 这个知识点和中断有什么区别？
* 能不能用更适合考研答题的方式再讲一遍？
* 给我出一道同知识点的变式题

追问输入区应保持轻量：保留多行输入框与发送按钮，去掉没有真实行为的 chip。快捷入口应面向后续 agent action，例如“归纳错因”“换种讲法”“生成相似题”“加入复习计划”，当前可先注入 prompt 或直接发送。

追问支持附图作为后续多模态 agent 的入口。前端只允许选择图片，Go 后端负责接收、校验和保存附件，并把附件元数据随聊天记录沉淀；Python 多模态追问可以后续在此基础上接入。

## 5.7 错题沉淀

题目保存后需要具备后续运营价值：

* 可检索
* 可筛选
* 可复习
* 可再次追问
* 可按状态流转

---

# 6. 信息架构

建议围绕“题目”组织数据，而不是围绕“会话”组织数据。

## 一级对象

* 题目 Question
* 题目资源 QuestionAsset
* 用户作答 UserAnswer
* 题目解析 Analysis
* 对话消息 ChatMessage
* 分类 Category
* 标签 Tag
* 题目状态 ReviewState

## 题目生命周期

1. uploaded
2. ocr_processing
3. pending_review
4. ready_for_answer
5. analysis_processing
6. analyzed
7. archived
8. mastered / review_later

这比“上传后直接存库”更合理，因为 OCR 与 AI 都可能失败或需要人工校准。

---

# 7. 技术架构总览

本次规划调整后，项目采用“双后端职责分离”架构。

## 7.1 核心业务端

技术栈：

* Go
* Gin
* MySQL
* Redis

职责：

* 对前端提供统一 REST API
* 用户管理与鉴权
* 题目、标签、分类、作答、状态流转 CRUD
* 文件上传后的资源登记
* 调用 Python AI 服务
* 接收 AI 结果并持久化
* 作为前端唯一访问入口

## 7.2 AI 算法端

技术栈：

* Python
* FastAPI

职责：

* 接收图片文件或图片 URL
* 执行 OCR
* 清洗 OCR 文本并恢复题目结构
* 调用 LLM 生成解析
* 返回标准 JSON
* 不直接暴露给前端
* 不直接承担主业务数据库读写

## 7.3 前端

技术栈建议：

* React + TypeScript
* React Router
* Zustand
* TanStack Query
* `shadcn/ui + Tailwind CSS`

职责：

* 工作台页面渲染
* 上传题目
* 展示识别状态与解析状态
* 用户作答、追问、编辑校准
* 轮询题目状态与解析状态

---

# 8. 服务边界与交互原则

## 8.1 为什么采用 Go + Python 分工

原因：

* Go 更适合做稳定 API 网关、状态机、CRUD 与用户管理
* Python 在 OCR、LLM、推理链路、第三方 AI SDK 上生态更成熟
* 这样可以避免把 AI 逻辑、用户逻辑、数据库逻辑混在一个服务里

## 8.2 必须遵守的边界

* 前端只能调用 Go API，不能直接调用 Python
* Go 负责业务主流程与状态管理
* Python 只返回算法结果，不负责业务决策
* AI 服务要被视为内部依赖，而不是业务主系统

## 8.3 推荐部署形态

```text
Frontend (React)
    ↓
Go API Server (Gin)
    ↓
-------------------------------------------
| MySQL | Redis | Object Storage | FastAPI |
-------------------------------------------
```

说明：

* 图片文件建议先由 Go 存入对象存储
* Python 优先通过内部 URL 拉取资源
* 小文件调试阶段可直接用 multipart 文件透传

---

# 9. Go 与 Python 的 API 契约设计

## 9.1 总体建议

Go 与 Python 之间采用内部 RESTful API，原因：

* 简单直接，易于调试
* 你当前缺少微服务经验，REST 比消息队列更容易落地
* 后续如果任务量变大，再在 Go 内部接入队列即可

内部接口建议前缀：

* `/internal/v1/ocr`
* `/internal/v1/analyze`
* `/internal/v1/health`

## 9.2 图片传输建议

建议分阶段：

### MVP 阶段

Go 直接把图片文件转发给 Python：

* 实现快
* 调试简单
* 适合单题处理

### 稳定阶段

Go 先把图片存对象存储，再把 `fileUrl` 和元信息传给 Python：

* Python 服务更轻
* 避免大文件在服务间重复传输
* 更利于重试与回放

结论：

* 本项目先保留两种模式
* 生产推荐 `fileUrl` 模式
* 本地联调保留 multipart 直传模式

## 9.3 OCR 接口契约

`POST /internal/v1/ocr/parse`

请求示例一：multipart 文件上传

```http
POST /internal/v1/ocr/parse
Content-Type: multipart/form-data
```

表单字段：

* `file`: 图片文件
* `question_id`: 业务题目 ID
* `source_type`: image
* `trace_id`: 链路追踪 ID

请求示例二：JSON URL 模式

```json
{
  "questionId": 123,
  "imageUrl": "https://storage.example.com/questions/q123.png",
  "sourceType": "image",
  "traceId": "trace_abc_001"
}
```

响应示例：

```json
{
  "traceId": "trace_abc_001",
  "questionId": 123,
  "status": "completed",
  "rawText": "1. 下列关于系统调用的说法...",
  "structuredQuestion": {
    "stem": "下列关于系统调用的说法，正确的是：",
    "questionType": "single_choice",
    "options": [
      { "key": "A", "content": "..." },
      { "key": "B", "content": "..." },
      { "key": "C", "content": "..." },
      { "key": "D", "content": "..." }
    ],
    "assets": [],
    "suggestedAnswer": "D"
  },
  "warnings": [],
  "cost": {
    "ocrMs": 1830,
    "parserMs": 620
  }
}
```

## 9.4 解析接口契约

`POST /internal/v1/analyze/question`

请求示例：

```json
{
  "questionId": 123,
  "traceId": "trace_abc_002",
  "question": {
    "stem": "下列关于系统调用的说法，正确的是：",
    "questionType": "single_choice",
    "options": [
      { "key": "A", "content": "..." },
      { "key": "B", "content": "..." },
      { "key": "C", "content": "..." },
      { "key": "D", "content": "..." }
    ]
  },
  "userAnswer": "B",
  "context": {
    "subject": "操作系统",
    "tags": ["408", "系统调用"]
  }
}
```

响应示例：

```json
{
  "traceId": "trace_abc_002",
  "questionId": 123,
  "status": "completed",
  "analysis": {
    "answer": "D",
    "summary": "本题考查系统调用的执行过程与用户态/内核态切换。",
    "knowledgePoints": ["系统调用", "用户态与内核态", "中断入口"],
    "steps": [
      "先判断系统调用会触发从用户态进入内核态。",
      "再判断处理过程中需要保存现场并切换执行上下文。",
      "结合选项比较，只有 D 符合。"
    ],
    "optionAnalysis": {
      "A": "错误，原因是...",
      "B": "错误，原因是...",
      "C": "错误，原因是...",
      "D": "正确，原因是..."
    },
    "pitfalls": [
      "容易把系统调用与普通函数调用混淆",
      "容易忽略用户态到内核态切换"
    ],
    "reviewAdvice": [
      "复习系统调用、中断、异常三者区别"
    ]
  },
  "cost": {
    "llmMs": 2940
  }
}
```

## 9.5 错误响应约定

Python 服务内部错误建议统一格式：

```json
{
  "traceId": "trace_abc_002",
  "status": "failed",
  "error": {
    "code": "OCR_TIMEOUT",
    "message": "ocr processing timeout"
  }
}
```

Go 接到错误后：

* 更新题目状态为 `ocr_failed` 或 `analysis_failed`
* 保留错误信息便于前端提示
* 前端允许用户重试或手动编辑

---

# 10. 前端、Go、Python 之间的同步与异步建议

## 10.1 结论

推荐采用：

* 前端到 Go：异步体验优先
* Go 到 Python：服务间同步调用
* 前端拿结果：轮询为主，WebSocket 为后续增强

## 10.2 为什么不建议前期全同步

原因：

* OCR 和 LLM 都是明显的慢操作
* 同步阻塞会让上传接口等待过久
* 用户体验差，失败重试也难做

## 10.3 推荐链路

### 导入题目链路

1. 前端上传题目到 Go
2. Go 立即创建 `question` 与 `job` 记录
3. Go 返回 `questionId`、`jobId`、当前状态
4. Go 在后台调用 Python OCR
5. 前端轮询题目状态或任务状态
6. OCR 完成后前端刷新题目详情

### 生成解析链路

1. 前端发起“生成解析”
2. Go 更新状态为 `analysis_processing`
3. Go 同步调用 Python 获取解析结果
4. 如果耗时还能接受，Go 完成后直接入库
5. 前端轮询 `analysisStatus`

## 10.4 为什么 Go 到 Python 先用同步调用

原因：

* 架构更简单
* Go 便于在一个后台 goroutine 中直接编排
* 日志链路清楚
* 你当前阶段不必马上引入消息队列

## 10.5 前端轮询与 WebSocket 的选择

### MVP 推荐

轮询。

建议：

* 上传后以约 1.2 秒起步、最多退避到 8 秒的轮询读取题目状态
* OCR/解析完成或失败后停止轮询

原因：

* 前端实现简单
* 服务端实现简单
* 对当前单题工作台场景足够

### 后续增强

当你要做以下能力时，再考虑 WebSocket：

* 批量导入进度
* 长时间排队任务
* 页面内多任务并发状态推送

结论：

* MVP 用“异步任务 + 状态轮询”
* 不建议一开始就上 WebSocket

---

# 11. 对外业务 API 草案

建议统一前缀：`/api/v1`

## 11.1 上传题目资源

`POST /api/v1/questions/import`

用途：上传图片或原始文本，创建单题录入任务。该接口不接受 PDF。

返回示例：

```json
{
  "jobId": "job_xxx",
  "questionId": 123,
  "status": "ocr_processing"
}
```

## 11.2 获取题目详情

`GET /api/v1/questions/{id}`

返回示例：

```json
{
  "id": 123,
  "stem": "执行系统调用的过程涉及...",
  "questionType": "single_choice",
  "options": [
    { "key": "A", "content": "..." },
    { "key": "B", "content": "..." }
  ],
  "reviewState": "pending_review",
  "ocrStatus": "completed",
  "analysisStatus": "completed"
}
```

## 11.3 获取任务状态

`GET /api/v1/jobs/{jobId}`

返回示例：

```json
{
  "jobId": "job_xxx",
  "questionId": 123,
  "type": "ocr",
  "status": "completed"
}
```

## 11.4 更新题目校准结果

`PATCH /api/v1/questions/{id}`

用途：用户修正 OCR 结果、答案、分类等。

## 11.5 提交作答

`POST /api/v1/questions/{id}/answer`

## 11.6 生成题目解析

`POST /api/v1/questions/{id}/analyze`

返回示例：

```json
{
  "analysisId": 888,
  "status": "analysis_processing"
}
```

## 11.7 获取题目解析

`GET /api/v1/questions/{id}/analysis`

## 11.8 题目对话

`POST /api/v1/questions/{id}/chat`

请求示例：

```json
{
  "message": "为什么 B 不对？"
}
```

## 11.9 题库列表

`GET /api/v1/questions`

建议支持筛选：

* 学科
* 题型
* 状态
* 标签
* 关键词
* 时间范围

## 11.10 分类与标签

* `GET /api/v1/categories`
* `POST /api/v1/categories`
* `GET /api/v1/tags`
* `POST /api/v1/tags`

约束：

* category 是单值学科维度；题目更新时只能设置一个已存在的顶层 category，传 `null` 表示未分类。
* tags 是多值知识点维度；`POST /api/v1/questions/{id}/tags` 整体替换标签集合，Go 会去重并拒绝不存在的 tag ID。
* 分类创建/更新不接受 `parentId`，避免把章节或 TCP、UDP 等知识点误建成 category。

AI 建议契约（随解析结果返回的可选字段）：

```json
{
  "taxonomySuggestion": {
    "categoryName": "计算机网络",
    "tagNames": ["TCP", "拥塞控制"],
    "confidence": 0.86
  }
}
```

Python 只负责产生这段结果；Go 负责将其纳入解析记录、校验当前用户可见候选是否存在，并在题目没有人工 taxonomy 时通过上述分类/标签业务接口自动生效。用户仍可在题目详情中手动改分类或标签；没有可用大类时保持未分类并提示原因。

## 11.11 学习状态与推荐练习

当前实现：

* `GET /api/v1/questions/{id}/learning-state`
* `PATCH /api/v1/questions/{id}/learning-state`
* `POST /api/v1/questions/{id}/learning-state/generate`
* `GET /api/v1/recommendations/practice`

`learning-state` 保存题目级学习信息：`masteryLevel`、`wrongCount`、`correctStreak`、`lastPracticedAt`、`nextReviewAt`、`mistakeReason`、`weaknessTags`、`reviewAdvice`。

推荐接口一次返回五组候选：

* `today_review`：`next_review_at <= now`
* `recent_wrong`：`wrong_count > 0`
* `weak_points`：按 `weaknessTags` 聚合
* `new_questions`：未练习或未初始化学习状态
* `mixed_random`：混合随机抽取

接口只返回 `questionIds`、数量和推荐原因；创建练习仍复用 `POST /api/v1/practice-sessions`，避免把推荐策略和会话生命周期混在一起。

---

# 12. 数据模型建议

## 12.2 categories

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| name | varchar | 分类名 |
| parent_id | bigint nullable | 历史兼容字段；当前新建/更新必须为 null，分类表示顶层学科 |
| created_at | timestamp | 创建时间 |

## 12.3 questions

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| category_id | bigint nullable | 主分类 |
| stem | text | 题干 |
| question_type | varchar | 题型 |
| correct_answer | text nullable | 正确答案；支持主观题长文本、公式与代码 |
| user_answer | text nullable | 用户最近一次作答 |
| review_state | varchar | 学习状态 |
| ocr_status | varchar | uploaded/processing/completed/failed |
| analysis_status | varchar | queued/processing/completed/needs_review/failed |
| source_type | varchar | image/manual |
| raw_ocr_text | longtext nullable | OCR 原始文本 |
| structure_warnings | json nullable | OCR 后结构化纠错 warning |
| structure_confidence | float nullable | OCR/结构化置信度 |
| parse_source | varchar | 结构化来源，如 rules/llm_refined/manual_corrected |
| created_at | timestamp | 创建时间 |
| updated_at | timestamp | 更新时间 |

## 12.4 question_options

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| question_id | bigint | 题目 ID |
| option_key | varchar | A/B/C/D |
| content | text | 选项内容 |
| sort_order | int | 排序 |

## 12.5 question_assets

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| question_id | bigint | 题目 ID |
| asset_type | varchar | image/crop |
| file_url | varchar | 存储地址 |
| meta_json | json | 元信息 |

## 12.6 analyses

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| question_id | bigint | 题目 ID |
| provider | varchar | 使用的模型提供方 |
| answer | text nullable | 解析答案；支持主观题长文本、公式与代码 |
| content_json | json | 结构化解析结果 |
| created_at | timestamp | 创建时间 |

## 12.7 jobs

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| job_id | varchar | 任务业务 ID |
| question_id | bigint | 题目 ID |
| job_type | varchar | ocr/analyze |
| status | varchar | queued/processing/completed/failed |
| error_code | varchar nullable | 错误码 |
| error_message | text nullable | 错误信息 |
| started_at | timestamp nullable | 开始时间 |
| finished_at | timestamp nullable | 结束时间 |
| created_at | timestamp | 创建时间 |

## 12.8 chat_messages

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| question_id | bigint | 题目 ID |
| role | varchar | user/assistant/system |
| message | text | 消息内容 |
| created_at | timestamp | 创建时间 |

## 12.9 tags / question_tags

`tags` 与 `question_tags` 组成 Question 到细粒度知识点的多对多关系。一个 Question 只能通过 `questions.category_id` 绑定一个 category，但可以绑定多个 tag。category 用于稳定的学科分组，tag 用于知识点检索、复习推荐和 AI 建议；两者不能混用。

## 12.10 question_learning_states

以 `Question` 为业务核心，为每道题维护一条学习状态：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| question_id | bigint | 题目 ID，唯一索引 |
| mastery_level | int | 掌握度，MVP 使用 0-5 |
| wrong_count | int | 累计答错次数 |
| correct_streak | int | 连续答对次数 |
| last_practiced_at | timestamp nullable | 最近练习时间 |
| next_review_at | timestamp nullable | 下次建议复习时间 |
| mistake_reason | text nullable | agent 归纳并允许用户修订的错因 |
| weakness_tags | json | 薄弱知识点或策略标签 |
| review_advice | json | 可编辑复习建议 |
| created_at | timestamp | 创建时间 |
| updated_at | timestamp | 更新时间 |

作答后的最小更新规则：

* 答错：`wrong_count + 1`，`correct_streak = 0`，掌握度下降或保持低位，`next_review_at` 提前。
* 答对：`correct_streak + 1`，掌握度最高提升到 5，并延后 `next_review_at`。
* 主观题、简答题、论述题、计算题：进入 `manual_required`，只更新最近练习时间和短期复习提醒，不自动判定掌握。
* 推荐服务只读取学习状态和题目维度，不直接修改作答记录。

---

# 13. 推荐目录结构

## 13.1 Go 核心业务端

```text
backend/
  cmd/
    server/
      main.go
  internal/
    handlers/
    services/
    repository/
    models/
    integrations/
      ai/
        client.go
  pkg/
    response/
```

## 13.2 Python AI 算法端

```text
ai-service/
  app/
    api/
      routes/
        health.py
        ocr.py
        analysis.py
    core/
      config.py
    schemas/
      common.py
      ocr.py
      analysis.py
    services/
      ocr_service.py
      analysis_service.py
    main.py
  requirements.txt
```

---

# 14. FastAPI 脚手架代码示例

下面这段代码用于说明推荐的 Python 服务最小骨架。它包含：

* 健康检查
* 接收图片文件
* 模拟 OCR 耗时
* 模拟 LLM 解析
* 返回标准 JSON 结构

```python
from __future__ import annotations

import asyncio
import uuid
from typing import Literal

from fastapi import APIRouter, FastAPI, File, Form, HTTPException, UploadFile
from pydantic import BaseModel, Field


class OptionItem(BaseModel):
    key: str
    content: str


class StructuredQuestion(BaseModel):
    stem: str
    questionType: Literal[
        "single_choice", "multiple_choice", "true_false", "fill_blank",
        "subjective", "short_answer", "essay", "calculation"
    ]
    options: list[OptionItem] = Field(default_factory=list)
    assets: list[dict] = Field(default_factory=list)
    suggestedAnswer: str | None = None


class OCRResponse(BaseModel):
    traceId: str
    questionId: int
    status: Literal["completed"]
    rawText: str
    structuredQuestion: StructuredQuestion
    warnings: list[str] = Field(default_factory=list)
    cost: dict[str, int]


class AnalysisRequest(BaseModel):
    questionId: int
    traceId: str
    question: StructuredQuestion
    userAnswer: str | None = None
    context: dict = Field(default_factory=dict)


class AnalysisPayload(BaseModel):
    answer: str
    summary: str
    knowledgePoints: list[str]
    steps: list[str]
    optionAnalysis: dict[str, str]
    pitfalls: list[str]
    reviewAdvice: list[str]


class AnalysisResponse(BaseModel):
    traceId: str
    questionId: int
    status: Literal["completed"]
    analysis: AnalysisPayload
    cost: dict[str, int]


async def simulate_ocr(filename: str) -> tuple[str, StructuredQuestion]:
    await asyncio.sleep(2)
    raw_text = f"识别自文件 {filename}：下列关于系统调用的说法，正确的是？ A.... B.... C.... D...."
    structured = StructuredQuestion(
        stem="下列关于系统调用的说法，正确的是：",
        questionType="single_choice",
        options=[
            OptionItem(key="A", content="系统调用不会进入内核态"),
            OptionItem(key="B", content="系统调用与普通函数调用完全一致"),
            OptionItem(key="C", content="系统调用不会引发上下文切换"),
            OptionItem(key="D", content="系统调用通常需要从用户态切换到内核态"),
        ],
        suggestedAnswer="D",
    )
    return raw_text, structured


async def simulate_analysis(question: StructuredQuestion, user_answer: str | None) -> AnalysisPayload:
    await asyncio.sleep(2)
    return AnalysisPayload(
        answer="D",
        summary="本题考查系统调用的执行机制和特权级切换。",
        knowledgePoints=["系统调用", "用户态与内核态", "中断入口"],
        steps=[
            "先判断系统调用是否需要陷入内核。",
            "再判断执行期间是否涉及现场保护。",
            "比较四个选项，只有 D 正确。",
        ],
        optionAnalysis={
            "A": "错误，系统调用通常要进入内核态。",
            "B": "错误，系统调用涉及特权切换，不等同于普通函数调用。",
            "C": "错误，系统调用过程通常伴随执行上下文切换。",
            "D": "正确，符合系统调用的基本执行过程。",
        },
        pitfalls=[
            "把系统调用误认为普通函数调用。",
            "忽略用户态和内核态的边界。",
        ],
        reviewAdvice=[
            "复习系统调用、中断、异常三者之间的关系。",
            "结合操作系统进程切换流程再看一遍。",
        ],
    )


router = APIRouter(prefix="/internal/v1")


@router.get("/health")
async def health() -> dict[str, str]:
    return {"status": "ok"}


@router.post("/ocr/parse", response_model=OCRResponse)
async def parse_ocr(
    question_id: int = Form(...),
    trace_id: str = Form(default_factory=lambda: uuid.uuid4().hex),
    source_type: str = Form(default="image"),
    file: UploadFile = File(...),
) -> OCRResponse:
    if not file.content_type or not file.content_type.startswith("image/"):
        raise HTTPException(status_code=400, detail="only image upload is supported in mvp")

    raw_text, structured = await simulate_ocr(file.filename)
    return OCRResponse(
        traceId=trace_id,
        questionId=question_id,
        status="completed",
        rawText=raw_text,
        structuredQuestion=structured,
        warnings=[],
        cost={"ocrMs": 2000, "parserMs": 300},
    )


@router.post("/analyze/question", response_model=AnalysisResponse)
async def analyze_question(payload: AnalysisRequest) -> AnalysisResponse:
    analysis = await simulate_analysis(payload.question, payload.userAnswer)
    return AnalysisResponse(
        traceId=payload.traceId,
        questionId=payload.questionId,
        status="completed",
        analysis=analysis,
        cost={"llmMs": 2000},
    )


app = FastAPI(title="ErroNotebook AI Service", version="0.1.0")
app.include_router(router)
```

说明：

* 真正落地时应拆到 `schemas`、`services`、`routes`
* 上传文件应增加大小、类型校验
* 真实 OCR 和 LLM 调用应放到 service 层

---

# 15. Gin 调用 Python 微服务的 Service 示例

下面代码示例展示 Go 服务如何：

* 接收内部业务参数
* 把图片文件转成 multipart 请求发给 FastAPI
* 解析 Python 返回
* 将结构化结果交回上层业务

```go
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type OptionItem struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

type StructuredQuestion struct {
	Stem            string       `json:"stem"`
	QuestionType    string       `json:"questionType"`
	Options         []OptionItem `json:"options"`
	Assets          []any        `json:"assets"`
	SuggestedAnswer string       `json:"suggestedAnswer"`
}

type OCRResponse struct {
	TraceID            string             `json:"traceId"`
	QuestionID         int64              `json:"questionId"`
	Status             string             `json:"status"`
	RawText            string             `json:"rawText"`
	StructuredQuestion StructuredQuestion `json:"structuredQuestion"`
	Warnings           []string           `json:"warnings"`
	Cost               map[string]int     `json:"cost"`
}

func (c *Client) ParseQuestionImage(
	ctx context.Context,
	questionID int64,
	traceID string,
	sourceType string,
	filePath string,
) (*OCRResponse, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open image file: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("question_id", strconv.FormatInt(questionID, 10)); err != nil {
		return nil, fmt.Errorf("write question_id: %w", err)
	}
	if err := writer.WriteField("trace_id", traceID); err != nil {
		return nil, fmt.Errorf("write trace_id: %w", err)
	}
	if err := writer.WriteField("source_type", sourceType); err != nil {
		return nil, fmt.Errorf("write source_type: %w", err)
	}

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("copy file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/v1/ocr/parse",
		&body,
	)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ai service: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ai response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai service returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var result OCRResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("unmarshal ai response: %w", err)
	}

	return &result, nil
}
```

工程实践建议：

* `Client` 放在 `backend/internal/integrations/ai/client.go`
* 由 `services` 层调用，不要在 handler 里直接拼 HTTP 请求
* `traceId`、超时、错误码都要保留
* 后续可再补 `AnalyzeQuestion` 方法

---

# 16. 非功能要求

## 可用性

* OCR 失败时必须支持人工编辑
* AI 失败时不能影响题目入库
* 上传、识别、解析状态需要可视化

## 性能

* 单题图片上传后应尽快返回任务状态
* OCR 和解析建议异步触发、状态轮询
* Go 调 Python 的单次内部调用应设置超时
* 题库列表支持分页和筛选

## 可维护性

* OCR、解析、对话三个能力分层
* Go 和 Python 的职责边界清晰
* 数据结构优先结构化，不要把所有信息塞进一段文本
* 保留原始 OCR 结果，便于后续纠错与回放

## 安全性

* 文件上传需要类型和大小校验
* Go 作为唯一业务入口，Python 内部接口应限制在内网或网关后

---

# 17. 开发优先级建议

## P0

* 单实例数据模型与本地部署
* 上传图片
* Go 调 Python OCR
* 题目详情展示与作答
* AI 标准解析
* 围绕当前题的对话
* 保存到错题库

## P1

* `QuestionLearningState` 学习状态模型
* 作答后掌握度与复习时间更新
* 错因归纳与薄弱标签
* 今日复习、最近错题、薄弱专项、新题巩固、随机混合推荐接口
* 做题模式的推荐入口与推荐原因展示

## P2

* 知识点图谱
* 相似题与专项训练生成
* 学习数据看板
* WebSocket 进度推送

---

# 18. 当前建议结论

本轮收敛后，实现基线如下：

1. 产品核心是“AI 错题复习工作台”，不是 PDF 试卷拆题工具
2. 前端仍是工作台 UI，不改成聊天产品
3. Go 负责主业务、API、CRUD、状态和网关
4. Python 负责 OCR 与 LLM 等耗时 AI 任务
5. 八类题型的录入、作答、基础判分和练习会话继续保留
6. PDF 页面、入口、批次模型、接口与 `exam_parser` 已从 MVP 移除
7. 下一阶段直接进入学习状态模型、错因归纳、掌握度追踪与推荐题目接口

---

# 19. P0 单题辅导 Agent 实现基线（2026-09-07）

当前 P0 已将单题 Agent 收敛为四类显式动作：`diagnose_mistake`、`explain_alternative`、`hint`、`suggest_taxonomy`。Python 只接收 Go 构建的 `QuestionContext`，不凭题目 ID 查询业务库；Go 负责数据归属、版本指纹、保存和最终分类标签生效。

动作接口统一返回 `completed`、`needs_input`、`needs_review`、`failed`，错误包含 `code`、`message`、`retryable`、`traceId`。真实 provider 与 mock 必须显式标记，调用预算默认最多 3 次且最多 1 次结构修复。错因诊断只有在存在作答和证据时才能覆盖学习状态，AI 不直接改变掌握度和练习统计。

P0 的 Go 对外接入包括中栏动作代理和分类建议确认应用；Python 正式解析接口使用 multipart 的 `payload`/`file` 契约。图片追问、相似题生成和主观题辅助批改继续作为 P1，不进入当前交付承诺。

这套方案足够工程化，同时又不会因为过早引入复杂分布式设计而拖慢 MVP 落地。

# 20. 单实例数据模型与图片导入异步基线

本轮将“单实例直接完成单题闭环”落为实现约束：Go 不创建用户、账号、登录态、Cookie 或浏览器 session，前端请求无需 credentials 和 userId。当前部署的 MySQL 与本地对象存储就是唯一数据空间；需要隔离数据时使用不同部署实例。

Question 是业务组织中心。QuestionAsset、Analysis、Job、ChatMessage、BatchImport、PracticeSession、PracticeSessionQuestion、QuestionLearningState、AIProposal 都直接属于当前实例；对象存储 key 使用 `questions/{questionId}/...`。Category/Tag 是实例统一词表，AI taxonomy 在无人工选择时默认应用。

图片导入接口立即返回 `jobId`、`questionId` 和 `queued` 占位状态。Go worker 维护 `queued`、`processing`、`completed`、`needs_review`、`failed`，并通过 `processingStage` 区分 OCR 与 AI 分析；前端使用可取消、按任务去重、1.2 秒起步并带网络退避的轮询，最长等待 5 分钟。OCR 完成即刷新题干/题型/选项，AI 完成即刷新解析、taxonomy 建议和对话；解析未生成时显示“解析中”。切题、重复上传和卸载组件会取消旧轮询。旧数据库 `pending` Job 仍兼容接管。

图片分析优先使用 Go 转发的原图调用视觉 provider；视觉 provider 暂时不可达时，Python 在文本 provider 可用的前提下基于 OCR 结果降级解析并标记 `multimodal_fallback_to_ocr`，两者都不可用则明确失败并由 Go 重试，禁止把 mock 摘要伪装成真实结果。

本轮不引入账号体系、用户迁移、UserSkillProfile、向量库、知识图谱或 LangGraph。
