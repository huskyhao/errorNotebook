# ai-service 下一阶段计划与 Goal 指令

更新时间：2026-09-09。第 3 节 P0 指令已经执行；第 5 节是下一阶段待执行 Goal，不代表 P1 已完成。

## 1. 当前判断

本次按工作区实际代码规划，而不是沿用仓库早期“尚未编码”的判断。现有未提交修改属于当前基线，执行时必须保留。

已经存在的基础：

- OCR、规则结构化、按需 LLM 题面纠错。
- 文本解析，以及解析接口接收图片后的多模态分支。
- 围绕题目、用户答案和已有解析的文本追问。
- `taxonomySuggestion` 可选字段与提示词，Go 保存建议但不自动应用。
- Go 端学习状态、归纳错因入口、附件保存，以及后台任务 worker。

本次检查发现的主要提升点：

| 位置 | 现状 | 下一步 |
| --- | --- | --- |
| `backend/internal/services/question_service.go` | `GenerateLearningState` 调用规则函数拼接错因、标签和建议 | 接入有证据的 AI 错因诊断，Go 校验后保存 |
| `app/services/chat_service.py` | 模型异常被转换为抱歉文本，响应仍是 `completed` | 返回明确失败，支持业务端重试 |
| `app/services/openai_client.py`、`vision_service.py` | async 方法内部使用阻塞式网络请求 | 统一可取消、有限超时与重试的调用层 |
| `app/schemas/chat.py` | 没有附件字段，history role 是任意字符串 | 收紧上下文契约；真实图片追问放到下一批 |
| `app/services/analysis_service.py` | 有 JSON 校验；缺少统一修复、语义校验和候选 taxonomy 约束 | 增加共享结构化输出管线与质量门槛 |
| `ai-service/README.md` | 解析示例使用 JSON 请求，但当前正式路由要求 multipart | 以实际路由为准修正文档并增加契约验证 |

## 2. 推荐推进顺序

当前 Goal 聚焦 P0：可靠调用基础、单题 Agent 动作、最小业务接入和效果验收。完成后再单独启动 P1，避免一次性承诺过大。

| 顺序 | 交付内容 | 完成条件 |
| --- | --- | --- |
| P0-1 | 调用、错误、上下文、结构化输出基础 | 真实失败不伪装成功；超时、重试和并发有界；现有接口兼容 |
| P0-2 | 标准解析质量提升；`diagnose_mistake`、`explain_alternative`、`hint`、`suggest_taxonomy` 四类动作 | 每类动作有独立输入输出、校验和不足信息处理 |
| P0-3 | Go 接入和现有中栏动作展示 | 错因可保存；提示和换种讲法可使用；分类建议确认后生效 |
| P0-4 | 离线评测、可选真实模型验收、文档 | 有可复现结果，明确区分 mock、真实结果与未验证项 |
| P1-1 | 图片追问 | Go 校验并传递实际图片内容，Python 能回答图中信息；不能只传附件路径文字 |
| P1-2 | 相似题生成 | 每次生成 1 道结构化候选题，校验答案与解析，用户确认后由 Go 新建题目 |
| P1-3 | 主观题辅助批改 | 有评分依据时返回得分建议、缺失要点和不确定项；最终批改与学习状态更新仍由 Go 控制 |

P1 不属于下面指令的完成条件。暂不增加知识图谱、向量库、联网搜索、独立 Agent 平台、更多微服务或 PDF 主线。

## 3. 可直接使用的完整 Goal 指令

以下内容用于之后启动实现任务。本轮仅编写指令。

```text
请以“完成 ErroNotebook ai-service P0 单题辅导 Agent 能力升级，并接通现有 Go 业务闭环”为本次 goal。请实际完成实现、必要测试和文档，不只输出方案。严格按以下边界、顺序和验收条件执行。

一、理解基线与工作边界
1. 先阅读 AGENTS.md、project.md、webdesign.md、talk.md 最新记录、ai-service/AGENT_GOAL.md、ai-service 和 backend 的相关实现与测试。以当前工作区代码为实现基线，记录现有能力与缺口。
2. 保留已有未提交修改，不执行重置、清理或大范围覆盖。先运行与本次范围相关的现有检查，区分已有失败和本次引入的失败。
3. 核心实体始终是 Question。前端只访问 Go；Go 负责鉴权、数据归属、业务状态、任务编排与数据库；Python 只执行 OCR/LLM 与有界的 AI 工作流，返回结构化结果，不直接写主业务库。
4. 优先复用现有 client、schema、service、任务 worker、学习状态和中栏动作入口。使用单一轻量动作分发器与明确步骤；不为本次目标引入多 Agent 对话、无限自主循环或重量级框架。
5. 开始改接口前，将输入输出、兼容方式、状态、错误码和 Go 字段映射写入文档，然后实现。普通可逆实现自主推进；只有确实缺失的外部配置或冲突信息才需要用户补充。

二、P0-1：可靠调用和结构化输出基础
1. 抽出可复用的模型调用接口，支持现有文本与视觉 provider；不要固定新增模型或替换用户模型配置。消除 async 路径上的阻塞网络调用，管理连接生命周期、并发上限、取消和超时。
2. mock 与真实模式显式区分；真实模式配置缺失、provider 不可用时返回配置/依赖错误，禁止静默生成 mock 成功结果。兼容原有 auto 行为时必须清楚标记来源、降级原因与环境限制。
3. 统一错误结构，至少包含 code、message、retryable、traceId；区分配置错误、超时、限流、上游不可用、非法输出、请求不合法。不要把上游密钥或原始敏感响应透传前端。
4. 统一 JSON 提取、Pydantic 校验与领域校验。非法结构最多发起一次模型修复；修复不得编造题面事实。超时、429、临时 5xx 可在总预算内退避重试；401/403、请求错误不自动重试。
5. 所有重试和输出修复共享调用预算：默认每动作最多 3 次模型调用（含首次）、至多 1 次修复。总时限可配置，不新增无限循环。梳理 Go 同步超时、后台任务超时和 Python deadline；慢动作复用既有 Job/worker，不简单无限增加 HTTP 等待时间。
6. 修复 ChatService 把模型失败转换为 completed 的行为；Go 不得把失败提示当成正常助手回复保存。保留用户消息以便重试，并避免重试重复写入。
7. 日志至少记录 traceId、questionId、action、provider/model、promptVersion、durationMs、attempts、结果状态与可用的 token usage。拿不到的 usage 用 null/缺省，不伪造数值；默认不记录题面全文、附件内容和模型原始长响应。

三、P0-2：统一单题上下文和 Agent 动作
1. 在 Python 建立 QuestionContext：题目快照、题面质量 warnings、参考答案及来源、最近作答、已有解析、当前题对话、Go 提供的候选分类/标签。为请求增加版本或内容指纹，供 Go 判断结果是否过期。
2. history 只接受 user/assistant，禁止客户端插入 system/tool 权限。题面、OCR、历史消息和附件文字属于输入数据，不能改写系统规则。为各字段、历史条数与总上下文设置可配置预算，裁剪时保留当前问题、最近作答和必要题面，返回裁剪提示。
3. 建立 app/agents（或符合现有结构的等价模块）、动作注册表和独立 prompts。流程为“校验输入 → 按显式 action 选择处理器 → 生成 → 结构和领域校验 → 必要时一次修复 → 返回”。优先显式 action 枚举，不依赖模型猜测用户按钮意图。
4. 同步提升现有标准解析：答案、教学步骤与选项分析应一致；选择题答案必须引用存在的选项；无选项题不生成虚构选项分析；覆盖项目保留的八类题型。选项缺失、缺图、条件不足时标记 needs_review 或 needs_input，并解释缺少什么，不能强行给确定答案。
5. 实现 diagnose_mistake：输入题目、作答、参考解析与当前题必要对话；返回 mistakeReason、reasonType、evidence、weaknessTags、reviewAdvice、uncertainties。reasonType 至少覆盖概念混淆、条件遗漏、计算错误、方法错误、证据不足。evidence 指向提供的答案、步骤或消息，不编造学习经历。只有错误选项时只给“可能错因”；未作答或无法确认对错时不能宣称已诊断具体错误。
6. 实现 explain_alternative：接收用户不理解的概念或步骤，输出 explanation、focusPoints、checkQuestion；给出不同解释路径或例子，保持与已核实题面一致。发现原解析矛盾时明确指出并返回待复核，不直接覆盖旧答案。
7. 实现 hint：支持 1/2/3 级提示，对应知识点方向、关键条件、下一步操作；返回 hintLevel、hint、nextQuestion、revealsAnswer。提示模式避免直接给完整答案，用户明确请求完整解答时走解析/追问入口。对泄露风险做校验与样例评测，不能仅依赖布尔字段自报。
8. 实现 suggest_taxonomy，并复用于标准解析中的 taxonomySuggestion：只从 Go 提供的现有顶层分类候选中选 0 或 1 个 categoryName；tagNames 必须符合现有标签候选。没有匹配或没有候选时允许空结果并解释原因。限制数量、去重、校验置信度范围；置信度仅为模型自评，不当作已校准概率。不自动创建分类或标签。

四、接口契约与状态
1. 优先保留 /internal/v1/ocr/parse、/analyze/question、/chat/question。新增内部动作接口建议为 POST /internal/v1/agent/actions；如复用现有路由更合理，须写清映射，不同时维护两套重复逻辑。
2. 动作请求至少包含 traceId、questionId、action、context、params，以及题目/作答版本或内容指纹。action 仅允许上述四种；context 由 Go 构建，Python 不凭 questionId 查询业务数据库。
3. 响应包含 traceId、questionId、action、status、按 action 区分的类型化 result、warnings、error、meta。status 采用 completed、needs_input、needs_review、failed；补充问题和复核原因有明确字段。meta 记录实际 provider/model、版本、耗时、重试次数、可用 usage 与来源标记。
4. action 与 result 用枚举和判别联合校验，不能用任意 dict 掩盖契约。定义 HTTP 映射：非法输入 4xx；上游/超时失败 5xx 并带标准 error；needs_input/needs_review 是已处理的业务结果，使用 200 加明确状态。以最终契约同步 Go 实现。
5. 内部 AI 响应状态与数据库 Job 状态分开：Go 将 Job 收敛为既有终态，再保存结果质量或待补充状态，不让前端无限轮询；失败重试不能重复更新学习状态。
6. 对每个动作提供一个完整请求与响应示例，另给缺少作答、残缺题面、模型超时和非法 JSON 示例。校准 ai-service README 中分析接口的 multipart payload/file 请求方式，并验证文档示例符合实际路由。

五、P0-3：Go 与现有工作台接入
1. 扩展 backend/internal/integrations/ai 的类型和 client，Go 根据当前题的数据归属与版本构建上下文并调用动作接口。
2. 将现有“归纳错因”接到 diagnose_mistake。completed 且证据充分时按既有显式操作语义保存 mistakeReason、weaknessTags、reviewAdvice。AI 失败时保持旧值；不足信息的假设只能作为建议展示，不能覆盖已确认诊断；用户手动修订过的值不得静默覆盖。
3. AI 诊断不直接改变掌握度、正确率、错题次数、连续答对或复习日期，继续由 Go 的练习/学习状态规则更新。保存前校验题目与作答版本，过期结果不得覆盖新数据。
4. taxonomy 建议与实际分类标签分离。补齐用户“确认应用建议”的 Go 操作：重新校验现有分类、合法标签和数据归属，事务化保存；支持无匹配、候选已删除、重复点击等情况，不自动创建 taxonomy。
5. 让现有中栏可以使用换种讲法和分级提示，显示执行中、结果、待补充、待复核、失败重试。只做接入所需的最小前端调整，保留桌面三栏和当前右栏职责，不新增独立 Agent 页面。

六、P0-4：测试、评测与完成条件
1. 建立离线可跑的契约和故障测试，使用可注入 fake/mock provider，不要求 API Key。覆盖四动作成功；缺少作答；错误选项但无作答过程；答案/解析矛盾；残缺选项；题面指令注入与非法 role；长历史裁剪；无 taxonomy 匹配；超时、429、401、5xx；非法 JSON 修复成功/失败；调用预算耗尽；取消与并发；过期结果及重复保存。
2. 准备至少 20 个可追溯的人工编写或有授权的评测案例，覆盖八类题型与考研 408 四门学科，包含正常题和坏输入；每例有预期要点、不可出现内容、正确答案/不确定条件及人工审核准则。不要靠模型评价自己的结果作为唯一验收依据。
3. 评测报告分别记录 schema 成功率、答案/解析一致性、错因证据符合度、提示泄露、taxonomy 越界、失败状态正确性，以及真实模式下的耗时和可得 token 用量。固定样例回归中非法 taxonomy、mock 冒充真实、失败伪装完成、过期覆盖等契约违规必须为 0；真实内容质量如实报告，不虚构提升比例。
4. 执行 Python 相关测试与 Go 测试；改前端时执行现有前端检查/构建。至少增加 Go→Python 请求响应契约验证，不能只检查路由名存在。完成一次从中栏发起动作到 Go 保存/展示的闭环验证；如数据库或真实 provider 不可用，交付可重复的脚本与清楚标记的待验证项。
5. 若本地已有可用且获授权的 provider 配置，按有限调用预算完成真实样例验收；不读取或打印密钥值。未配置时先完成全部离线工作，不将 mock 的语义效果当成真实模型质量，也不宣称已完成真实端到端验收。
6. 同步 ai-service/README.md、必要的 .env.example、backend/API.md、project.md 与涉及的 webdesign.md；把本次变更和验证结论从 talk.md 第一行开始追加，旧记录整体保留。
7. 最终交付：能力清单、关键文件、契约与示例、测试命令及结果、评测报告、真实/mock 验证范围、已知限制与剩余阻塞。P0 仅在上述实现和必要验证完成后认定完成；外部阻塞须如实说明，不能用“代码已写”替代验证。

本次不实施 P1 图片追问、相似题生成和主观题辅助批改；完成 P0 后在 talk.md 给出这些能力的后续顺序即可。
```

## 4. 本轮使用方式

把上面代码块作为下一次实现任务的目标内容，也可以使用简短入口：

> 请将 `ai-service/AGENT_GOAL.md` 第 3 节完整指令设为本次 goal，并按其中 P0 范围实施、测试和验收；先检查当前工作区，保留已有修改。P1 留待后续。

这里提供的是自然语言目标内容，不依赖特定客户端的命令语法。本轮没有启动目标执行，也没有验证实际模型效果。

## 5. Taxonomy 闭环与 P1 完整 Goal 指令

以下指令用于下一次实现任务，范围包括：补齐 P0 遗留的 taxonomy 自动建议链路，以及 P1 图片追问、相似题生成、主观题辅助批改。分类和标签仍由用户确认后生效，不允许模型直接写业务库或自动创建 taxonomy。

```text
请以“完成 ErroNotebook taxonomy 建议闭环与 ai-service P1 单题辅导能力，并接通现有 Go 工作台和练习流程”为本次 goal。请实际完成实现、测试、评测和文档，不只输出方案。先检查当前工作区并保留已有修改；严格遵守 Question 核心实体、前端只访问 Go、Go 掌握业务状态和数据库、Python 只处理 OCR/LLM/多模态结构化结果的服务边界。

一、确认基线与控制范围
1. 先完整阅读 AGENTS.md、project.md、webdesign.md、talk.md 最新记录、ai-service/AGENT_GOAL.md、P0 评测报告，以及 ai-service、backend、frontend 相关实现和测试。以当前工作区代码为基线，不重置、不清理、不覆盖已有修改。
2. 先运行 P0 基线检查：Python 测试、Go 测试、前端构建和离线评测；记录已有失败。P1 实现不得回退 P0 的四动作、错误结构、调用预算、mock/真实来源区分、版本指纹和失败不伪装 completed 等约束。
3. 本轮只完成 taxonomy 建议闭环、真实图片追问、单道相似题候选、主观题辅助批改。不要引入知识图谱、向量库、联网搜索、PDF 主线、独立 Agent 平台、更多微服务或无限自主循环。
4. 所有生成结果都是受版本约束的 AI 建议。Python 不查询或写入业务数据库；Go 校验用户、数据归属、题目/作答版本、幂等性和最终持久化。任何过期结果都不得覆盖新题面、新作答或人工修改。
5. 开始编码前先在 backend/API.md 和 ai-service/README.md 写清请求、响应、状态、错误码、幂等键、版本字段和 Go/Python 字段映射，再按最终契约实现；不要同时维护两套重复逻辑。

二、先补齐 taxonomy 自动建议闭环
1. 修复标准解析链路没有传 categoryCandidates/tagCandidates 的问题。Go 在领取 analyze Job 时查询当前用户可用的顶层分类和合法标签，把候选名称及必要 ID 映射加入 AnalysisRequest.context；Python 只能从候选中选择，不能凭空返回新名称。
2. 标准解析成功时自动生成 taxonomySuggestion；也允许通过 suggest_taxonomy 显式重新生成建议。两条路径必须复用同一提示词、Pydantic 模型和领域校验，不得出现互相不一致的规则。
3. categoryName 只能为空或命中一个现有顶层分类；tagNames 只能命中现有标签，去重并限制数量。没有候选、没有匹配、题面信息不足时返回空建议和 reason，不把空建议伪装为成功分类。
4. mock 模式提供可预测、候选受限的契约结果并明确标记 source=mock；真实模式配置缺失或调用失败返回标准错误，不静默回退 mock。不得把 mock 的分类准确率当作真实模型效果。
5. Go 保存分析中的建议及其 sourceQuestionFingerprint、候选快照或候选版本、生成来源和时间。应用前重新加载题目及 taxonomy；候选已删除、题目已变更或建议过期时拒绝应用并返回明确状态。
6. 保留“建议与实际分类标签分离”。前端在解析卡片显示建议、来源、空结果原因、待确认/已过期/应用失败状态，并提供“生成/重新生成建议”“确认应用”“忽略”操作。只有用户确认后，Go 才在事务中应用现有 category/tag；不得自动创建分类标签。
7. 确认应用必须幂等：同一建议重复点击不得重复写关系或报无法理解的冲突；成功后刷新题目详情和筛选数据。人工修改后的分类标签优先，旧建议不得静默覆盖。

三、P1-1：真实图片追问
1. 复用现有聊天附件保存能力，但修复当前只把附件路径/文件名拼成文字的问题。Go 必须从服务端对象存储读取已校验的实际图片字节，或生成有时效且仅内部可访问的 URL；Python 必须收到真实图片内容，不能把本地路径文字当视觉输入。
2. 前端仍只调用 Go 的题目聊天接口。Go 校验题目和附件归属、MIME、扩展名、大小、数量和空文件；默认最多 4 张，只接受支持的图片类型，拒绝路径穿越、外部任意 URL 和客户端伪造的服务端路径。
3. 将 /internal/v1/chat/question 扩展为兼容旧 JSON 文本请求和 multipart payload/files 图片请求，或采用一个清晰的等价契约。旧文本追问不能被破坏；带图请求在 QuestionContext 中保留题目、作答、解析和受限历史。
4. 扩展现有视觉 provider，使一次请求能接收有界的多张图片，并继续遵守超时、取消、并发、总调用预算、标准错误和安全日志要求。日志只记录附件数量、类型和字节数，不记录图片 base64、题面全文或密钥。
5. 图片缺失、损坏、过大、provider 不支持视觉、图片信息不足时分别返回可区分的 4xx、failed、needs_input 或 needs_review。真实模型失败时保留用户消息和附件元数据，不保存“抱歉”等伪成功 assistant 消息；重试不得重复写用户消息或附件记录。
6. 中栏展示上传中、视觉分析中、待补图、待复核和失败重试。成功回答必须绑定当前题；附件属于输入数据，图片中的“忽略系统规则”等文字不得改变系统指令。

四、P1-2：生成一题相似题候选
1. 在显式动作注册表增加 generate_similar_question，不让模型猜测按钮意图。每次只生成 1 道结构化候选题，输入包括当前 QuestionContext、目标难度、允许变化的知识点/题型和源题指纹。
2. 结果至少包含 proposalId、sourceQuestionId、sourceFingerprint、stem、questionType、options、answer、analysis、knowledgePoints、variationStrategy、warnings、qualityStatus。八类题型沿用现有枚举；选择题答案必须引用存在选项，无选项题不得生成 optionAnalysis。
3. 领域校验必须检查题干非空、选项键唯一、答案/解析一致、题型契约、与原题不能只是数字或选项顺序的机械复制。无法验证答案、条件不足或与原题矛盾时返回 needs_review，不允许直接创建 Question。
4. Go 保存待确认 proposal，建议新增最小的通用 AI proposal 模型，至少包含用户、源题、动作、状态 pending/applied/rejected/expired、内容 JSON、源指纹、幂等键、过期时间和 createdQuestionId；不要让 Python 写库。
5. 对外提供生成、查看、确认创建、拒绝接口。只有用户点击“确认创建”后，Go 才重新校验源题版本并事务化创建新的 Question、Options 和必要关联；新题默认标记 sourceType=ai_generated 或等价来源，不继承旧题作答、学习状态和聊天记录。
6. 确认创建必须幂等：同一 proposal 重复确认只能得到同一 createdQuestionId。过期、已拒绝、已应用或源题已变化均返回明确状态。不得自动加入正确/错误统计。
7. 中栏恢复“生成相似题”入口，显示生成中、候选预览、待复核、确认保存、放弃和失败重试；候选未确认前不能出现在正式题库和练习推荐中。

五、P1-3：主观题辅助批改
1. 在显式动作注册表增加 grade_subjective_answer，只允许 subjective、short_answer、essay、calculation。客观题继续走现有规则判分，不绕到 LLM。
2. 输入必须包含用户作答、题目版本/指纹，以及可追溯的评分依据：人工 rubric、标准答案、参考解析或明确评分点至少一种。没有评分依据、未作答、题面残缺时返回 needs_input；不得凭空编造满分标准。
3. 类型化结果至少包含 suggestedScore、maxScore、criteriaResults、strengths、missingPoints、feedback、evidence、uncertainties、confidence、requiresHumanReview。每个评分项给出依据和得分范围；suggestedScore 必须在 0..maxScore，分项和总分一致。
4. AI 结果只是 grading suggestion。Go 校验练习会话/题目/作答归属和版本并保存建议；不得由 Python 直接更新 PracticeSessionQuestion、QuestionLearningState、掌握度、正确率、错题次数或复习日期。
5. 对外提供生成评分建议、查看、确认采用/人工修订接口。最终确认由 Go 执行并记录确认来源 ai_confirmed/manual、确认人和时间；AI 低置信度、有 uncertainties 或 requiresHumanReview=true 时必须要求人工确认。
6. 确认采用和人工修订必须幂等，且只作用于同一份作答版本。用户修改答案后旧评分建议自动过期；重试失败不能重复更新会话得分或学习状态。
7. 在练习结果或单题工作台的合适位置展示“AI 建议分数”而不是“最终得分”，显示评分依据、缺失要点、不确定项和确认控件；保留桌面三栏，不增加独立批改产品页面。

六、共享接口、状态与数据约束
1. 扩展 AgentAction 枚举和判别联合，加入 generate_similar_question、grade_subjective_answer；每个 action 必须对应独立 params/result 模型，禁止用任意 dict 规避契约。
2. 保持响应统一为 traceId、questionId、action、status、result、warnings、error、meta；status 只用 completed、needs_input、needs_review、failed。非法输入 4xx，上游/超时 5xx，needs_input/needs_review 使用 200。
3. 所有 P1 动作继续共享默认最多 3 次模型调用、最多 1 次结构修复和可配置总 deadline；图片数量/字节、上下文字符、历史条数和模型输出长度均有硬上限。
4. Go 的数据库 Job 状态与 AI 业务状态分离；长动作复用现有 Job/worker 或清晰扩展 job_type，确保任务最终收敛，不让前端无限轮询。重复请求使用幂等键，不重复创建 proposal、消息、题目或评分。
5. 默认不记录题面全文、用户完整作答、图片内容、模型原始长响应和密钥。日志记录 traceId、questionId、proposalId、action、provider/model、promptVersion、durationMs、attempts、状态、附件统计和可用 usage。

七、测试、评测和验收
1. 使用可注入 fake/mock provider 建立离线测试，不要求 API Key。taxonomy 覆盖：候选正常命中、无候选、无匹配、越界名称被拒绝、候选删除、题目过期、人工分类优先、重复确认幂等。
2. 图片追问覆盖：真实图片字节到达 Python、多图顺序、文本兼容、空图、伪 MIME、超限、对象不存在、视觉 provider 不可用、图片指令注入、超时/取消、失败重试不重复消息。至少有一个 Go→Python multipart 契约测试，不能只断言路由存在。
3. 相似题覆盖八类题型、答案/选项不一致、重复选项键、无选项题、源题机械复制、needs_review、过期 proposal、拒绝、重复确认只创建一个 Question。
4. 主观题批改覆盖四类主观题、无作答、无 rubric、分数越界、分项总分不一致、证据不足、答案更新使建议过期、人工修订、重复确认不重复更新学习状态。
5. 扩展离线评测集，至少新增 24 个有预期要点和禁出内容的 P1 案例：taxonomy 不少于 6、图片追问不少于 6、相似题不少于 6、主观题批改不少于 6。人工规则或授权数据优先，模型不得作为唯一裁判。
6. 报告分别统计 taxonomy 候选命中/越界、图片信息利用与失败状态、相似题 schema/答案一致性/非机械复制、批改依据覆盖/分数合法性/不确定项，以及真实模式耗时和可得 token。mock 与真实结果分开，不虚构提升比例。
7. 执行 Python 全量测试、Go 全量测试、前端测试或生产构建、Go→Python 契约测试及离线评测。若本地已有获授权 provider，按有限预算做真实图片追问、相似题和批改各至少 2 例；不得读取或打印密钥。没有 provider 或数据库时完成所有离线工作，并把真实端到端列为明确未验证项。
8. 同步 ai-service/README.md、.env.example、backend/API.md、project.md、webdesign.md 和必要迁移说明；把变更、测试结果、真实/mock 范围和后续限制从 talk.md 第一行置顶记录，旧内容整体保留。

八、完成条件与最终交付
1. taxonomy 必须做到“标准解析自动产生受候选约束的建议 → 中栏可见 → 用户确认后 Go 幂等应用”；不能再出现 Go 未传候选导致建议始终为空的链路缺口。
2. 图片追问必须证明 Python 收到并使用实际图片内容；只传路径、文件名、附件文字或 mock 回答不算完成。
3. 相似题必须先形成可持久化、可预览、可拒绝、可幂等确认的候选；模型直接创建正式 Question 不算完成。
4. 主观题批改必须有评分依据、结构化证据和人工确认边界；AI 直接改最终成绩或学习状态不算完成。
5. 最终报告提供能力清单、关键文件、迁移/接口示例、测试命令和结果、评测报告、真实/mock 验证范围、已知限制与外部阻塞。只有实现和必要验证均完成后才能宣布本 Goal 完成。
```

简短启动入口：

> 请将 `ai-service/AGENT_GOAL.md` 第 5 节完整指令设为本次 goal，完成 taxonomy 建议闭环与 P1 图片追问、相似题生成、主观题辅助批改；先检查并保留当前工作区修改，按文档实现、测试、评测和验收。
