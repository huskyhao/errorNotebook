# 2026-09-18 真实视觉 OCR 替换静默 Mock 降级

## 问题与根因

- 线上上传图片后出现固定的“进程同步与互斥”题目，确认该文本来自 `MockOCRBackend`，不是图片识别结果。
- Python 镜像未安装体积较大的 PaddleOCR；原 `OCR_BACKEND=auto` 在 PaddleOCR 初始化失败时吞掉异常并静默退回 Mock，导致假题面被当作成功结果保存。
- 随后的解析链路确实调用了真实模型，但它拿到的输入已经是 Mock 假题，因此形成“真实 AI 解答假题”的错误组合。

## 修复结论

- `auto` 现在优先使用已配置的 OpenAI-compatible 视觉模型完成题面转写；服务器显式使用 `OCR_BACKEND=vision`。
- 没有可用真实 OCR provider 时返回 `OCR_PROVIDER_UNAVAILABLE`，不再生成或保存 Mock 题目；Mock 仅在明确设置 `OCR_BACKEND=mock` 时启用。
- Go worker 的 OCR 请求改用 300 秒异步 AI client，不再受 15 秒交互请求超时限制。
- 新增视觉 OCR 选择、真实转写和无 provider 明确失败测试；Python 36 项测试与 Go 全量测试通过。

# 2026-09-18 2C2G 服务器部署与 NPM 接入

## 部署结论

- 目标服务器为 2 核、约 1.6 GiB 可用内存、40 GiB 系统盘，已有 Nginx Proxy Manager、Memos、Portainer、Uptime Kuma 等服务；不适合在服务器上并行执行 Node、Go、Python 镜像构建。
- 首次远程构建触发系统盘读写带宽上限，已停止并清理 3.8 GiB 可回收 BuildKit 缓存，未删除任何现有业务镜像或容器。
- 服务器已有有效的 2 GiB `/swapfile`，现已启用并设置 `vm.swappiness=10`，同时保留 `/etc/fstab` 自动挂载。
- 三张业务镜像改为在本机按 `linux/amd64` 构建，通过校验后的归档上传，服务器只执行 `docker load` 与 `docker compose up --no-build`。
- 增加 `docker-compose.server.yml`：MySQL buffer pool 限制为 128 MiB，各服务设置内存上限；前端直接加入外部 `npm-network`，NPM 使用 `erro-notebook:80` 反代；Go/AI 端口只绑定回环地址，前端不再发布宿主机端口。

## 网络边界

- 云安全组只需放行 Nginx Proxy Manager 已使用的 `80/tcp` 与 `443/tcp`。
- `81/tcp` 是 NPM 管理端口，不建议向公网开放；应仅对可信来源放行或通过其他安全通道访问。
- 不放行 `3100`、`18080`、`18001`、`3306`；MySQL 只在项目 Docker 网络内访问。

# 2026-09-14 Docker 一键启动与 GitHub 发布

## 本轮结论

- 已建立 `docker-compose.yml`，统一编排 React + Nginx 前端、Go + Gin 业务后端、Python + FastAPI AI 服务和 MySQL 8.4；启动顺序由健康检查约束，不依赖人工等待。
- 前端生产镜像通过 Nginx 提供 SPA，并将浏览器的 `/api/v1` 请求转发给 Go；Go 通过 Docker 私有网络访问 Python 和 MySQL，继续遵守“前端不直连 Python、Go 是唯一业务入口”的边界。
- MySQL 和上传文件分别使用 `mysql-data`、`uploaded-files` named volume 持久化；MySQL 不暴露宿主机端口，避免与本机已有 MySQL 冲突。
- Docker 默认启用 mock OCR / LLM，不配置密钥也能启动完整系统；真实 Provider 只通过根目录 `.env` 注入，`.env` 不进入 Git 或镜像构建上下文。
- 三个自建镜像均采用分层或精简构建，Go 与 Python 运行阶段使用非 root 用户；各服务的 `.dockerignore` 排除了本地缓存、依赖、上传目录、密钥和构建产物。

## 验证结果

- `docker compose config --quiet` 通过，三个自建镜像实际构建成功。
- `mysql`、`ai-service`、`backend`、`frontend` 四个容器均为 `healthy`。
- 宿主机请求 `http://localhost:3000/health`、`http://localhost:3000/api/v1/health`、`http://localhost:8080/health`、`http://localhost:8001/internal/v1/health` 均返回 HTTP 200。
- Go 全量测试通过；Python 离线测试 33/33 通过；前端 4 个测试套件、5/5 测试通过，生产构建和容器内生产构建均成功。

## 使用方式

```powershell
Copy-Item .env.example .env
docker compose up -d --build
docker compose ps
```

工作台入口为 `http://localhost:3000`。普通停止使用 `docker compose down`，不会删除数据库和上传文件；只有明确要清空本地数据时才使用 `docker compose down -v`。

# 2026-09-14 主观题 103 解析失败诊断与日志补强

## 诊断结论

- 题目 103 的 OCR 已完成；analyze Job `analyze_1789321470135462600` 在 3 次尝试中均已获得 AI 结果，但写入 `analyses` 时失败：`Error 1406 (22001): Data too long for column 'answer' at row 1`。
- 当前运行中 MySQL 的 `analyses.answer` 与 `questions.correct_answer` 仍是 `varchar(255)`，说明上一轮改为 `text` 的 Go 模型尚未通过服务重启执行 AutoMigrate。由于解析记录从未成功插入，查询 `/questions/103/analysis` 返回 `ANALYSIS_NOT_FOUND` 是后续现象，不是根因。
- 处理顺序确定为：重启 Go 服务执行字段迁移，确认两列为 `text`，再重试上述失败 Job；无需重新 OCR 或再次上传图片。

## 日志与前端改进

- Python 文件日志此前在 Uvicorn 已预装 logging handler 时可能因 `basicConfig` 不生效而出现空文件；现已强制安装终端与文件 handler，运行日志写入 `ai-service/app/.log/<UTC 启动时间>.log`。
- 解析结构或答案完整性失败现在记录 `analysis.validation_retry` / `analysis.validation_failed` 及安全错误码，不记录完整题干、答案或 API Key。
- 单题导入轮询遇到失败时，前端会读取对应 Job 并展示 `errorMessage`，不再只显示笼统的“AI 解析失败”。

## 验证

- Python mock 隔离环境 33/33 通过；Go `go test ./...`、`go vet ./...` 通过；前端 5/5 测试及生产构建通过。

# 2026-09-14 主观题完整参考答案修复

## 本轮结论

- 已确认主观题返回 `“参考答案见解析”` 的根因：解析链路只做 Pydantic/JSON 结构校验，Prompt 仅要求“参考结论”，没有验证 `answer` 是否为可独立阅读的完整作答。
- 主观题、简答题、论述题和计算题的 `answer` 现在禁止使用“见解析、略、待补充、待人工核对”等占位语；题干或评分点明确要求代码/伪代码时，答案必须包含代码证据。
- 文本模型首次输出不完整时会在既有调用预算内自动修复一次；仍不合格则返回 `INVALID_OUTPUT`。视觉模型输出不合格时会走既有 OCR 文本模型降级；Go worker 再做一层占位答案拦截，避免错误结果入库。
- `questions.correct_answer` 与 `analyses.answer` 从 `varchar(255)` 改为 `text`，由现有 GORM AutoMigrate 在服务启动时迁移，支持保存带公式、长文本和代码的主观答案。
- 工作台的 AI 答案和题目正确答案改用 Markdown 渲染，主观答案编辑框改为多行文本框；单选题仍使用原有短答案展示和交互。

## 验证结果与使用说明

- Python mock 隔离环境全量测试：33/33 通过，其中新增占位语拒绝、代码要求校验、自动修复重试回归用例。
- Go `go test ./...`、`go vet ./...` 通过，新增 worker 占位答案防御测试。
- 前端 4 个测试套件、5 个测试通过，生产构建通过；仅有既有 React Router v7 future flag 提示。
- 已生成的旧解析不会被静默改写；部署并重启 Go/Python 服务后，对旧题点击“重新解析”即可生成并覆盖完整参考答案，同时启动时会把答案列迁移为 `text`。
- 本轮没有调用真实付费 Provider；真实模型质量仍需用同一道题执行一次“重新解析”验收。

# 2026-09-14 非单选题链路与 Provider 设置审查

## 本轮结论

- 原设置页把 API Key、Base URL 和模型写入浏览器 `localStorage`，后端和 Python 从未读取，因此“可保存”不等于生效；本轮改为只读 Provider 状态页，由 Go 读取 Python 健康检查结果，密钥继续只从服务端 `.env`/secrets 注入。
- 设置接口只返回 provider、model、configured 和 `source=server_env`，不返回密钥原文或后四位；Go 对 Python 非 2xx 响应不再透传原始响应体，降低敏感信息进入前端/日志的风险。
- OCR 规则解析、解析 Prompt、追问 Prompt 和 Agent Prompt 已增加八类题型专用约定：多选答案为多个选项 key，判断答案为 `true/false`，填空增加 `blankAnswers`，主观/简答/论述/计算增加 `scoringPoints` 与 `rubric`。
- 工作台详情区补齐多选组合答案、判断题无选项兜底、填空/主观/简答/论述/计算文本作答及非选项解析展示；单选仍沿用原即时选择链路。

## 验证结果与边界

- Python `python -m pytest -q`：30/30 通过；Go `go test ./...`：通过；前端 `npm test -- --watchAll=false --runInBand`：4 个测试套件、5 个测试通过；`npm run build`：通过，仅保留 React Router 既有升级提示。
- 真实 provider/API Key 是否可用仍取决于本机服务端环境变量，以上测试不替代真实模型质量验收。
- 暂不做浏览器端 API Key 保存、用户级密钥加密存储、在线测试连接或账号绑定；这些与当前单实例服务端配置边界不一致。

# 2026-09-13 全项目复核与下一阶段建议：完成单实例 MVP 真实可用性收口

## 当前阶段判断

项目已经完成了单题导入、异步 OCR/解析、人工校准、作答、题目追问、错因归纳、分类标签、推荐练习、学习统计和错题归档的主要代码链路，不应再按“早期规划项目”推进。下一阶段的核心不是继续增加页面或 Agent 动作，而是把现有能力变成安全、可验证、可部署的完整产品。

本轮基线验证：

- Go `go test ./...` 通过，`go vet ./...` 通过。
- Python 26 个测试通过。
- 前端 3 个测试套件、4 个测试通过，生产构建通过。
- P0 20 例、P1 24 例评测数据可以加载，但当前 runner 只执行少量通用 mock smoke，不是逐例质量评测，也不代表真实 Provider 效果。

## 复核发现的优先问题

1. 设置页当前把文本模型和视觉模型的 API Key 明文保存到浏览器 `localStorage`，但这些配置没有接入 Go/Python，因此既不生效，也违背“Provider Key 只由服务端 `.env` 或 secrets 管理”的单实例边界。
2. 主观题采用 AI 建议分数后，数据库仍写入 `GradingStatus=manual_required`、`Status=ungraded`，前端继续表现为“待批改”，确认动作没有形成终态。
3. 自动化测试结构不均衡：前端只有 4 个组件/工具测试，没有真实浏览器闭环；Go 的 handler、repository、迁移和主要事务缺少集成测试；当前评测脚本没有逐条执行 `p0_cases.json` / `p1_cases.json` 的 expected/forbidden 约束。
4. 项目文档仍有旧口径：`AGENTS.md` 仍写“早期规划阶段”和用户/鉴权职责，`project.md`、`backend/API.md` 仍残留“当前用户可见”“越权”等多用户表述。
5. 当前没有 Docker Compose、服务健康依赖、数据卷/备份说明和 CI，自部署仍依赖手工配置三个服务。
6. `backend/internal/services/question_service.go`、`frontend/src/pages/QuestionWorkbenchPage.tsx` 和 `frontend/src/App.css` 已经偏大；在补齐行为测试后应按导入、任务轮询、题目编辑、Agent 动作和页面区域拆分，降低后续修改风险。

## 推荐下一阶段 Goal

建议下一阶段命名为：`完成 ErroNotebook 单实例 MVP 的安全配置、真实验收与自部署收口`。

### P0：先修正确性和配置安全

1. 移除设置页 API Key 输入和 `localStorage` 持久化。设置页改为读取 Go 暴露的只读 Provider 状态，仅展示文本/视觉 Provider、模型、是否配置和 mock/real 状态；Key 继续由服务端 `.env` 或 Docker secrets 注入，不返回前端。
2. 修复主观题评分状态：至少区分 `manual_required`、`ai_suggested`、`confirmed`、`manually_adjusted`；确认后刷新做题结果并停止显示“待批改”。是否更新学习状态必须有明确规则，不能因 AI 分数自动推断掌握度。
3. 将 CORS 改为环境变量允许列表，生产模式不再反射任意 Origin；Go 中用于调试的 `/api/v1/internal/ai/*` 应可通过配置关闭或限制为本机/内部网络。
4. 同步 `AGENTS.md`、`project.md`、`backend/API.md`、README 和页面文案，彻底统一为单实例单租户、无注册登录、Provider 服务端配置。

### P1：建立真实质量和浏览器闭环

1. 将 P0/P1 runner 改成逐例执行：每一例构造真实 `QuestionContext`，执行对应 action，校验 expected/forbidden，输出通过率、失败案例、provider/model、耗时和 token；mock 与真实结果分开报告。
2. 建立 10～20 道获授权的 408 金标样例，覆盖四门学科、选择题、计算题、残缺 OCR、含图题和主观题。有限预算运行真实 OCR/LLM，人工记录题干恢复准确性、答案正确性、解析可用性、taxonomy 命中、耗时和失败原因。
3. 增加最小 Playwright 端到端测试，至少覆盖：图片导入与轮询、OCR 待校准、解析完成、切题不串数据、追问、失败重试、创建练习、提交结果、主观题确认、归档筛选。
4. 为 Go 增加测试数据库上的路由/事务测试，重点覆盖迁移幂等、分类标签去重、Job 重试与租约、proposal 重复确认/过期、主观题确认状态和删除关联完整性。

### P2：完成自部署交付

1. 增加前端、Go、Python Dockerfile 与根目录 `docker-compose.yml`，提供 MySQL、数据卷、健康检查、启动依赖和内部网络；Python 默认不直接暴露公网。
2. 增加 GitHub Actions，固定执行 Python 测试、Go test/vet、前端测试/构建和离线评测。
3. README 补最短启动、Provider 配置、数据备份/恢复、日志查看、升级迁移和停止服务说明。
4. 在 E2E 与集成测试保护下拆分超大文件，不在测试建立前做大规模重构。

## 完成标准

- 浏览器不保存或回显任何 Provider Key，设置页展示的状态与实际后端配置一致。
- 主观题人工采用评分后进入明确终态，刷新页面后状态和分数保持一致。
- 真实 408 样例有可复现报告，明确区分真实、mock 和未验证项。
- Playwright 能自动跑通核心闭环，Go 关键事务有集成测试。
- `docker compose up` 可以启动完整系统，健康检查、数据卷和备份步骤可复现。
- 全部文档不再残留登录、用户隔离或前端保存 API Key 的旧口径。

## 暂不建议做

- 暂不引入 LangGraph、向量库、知识图谱、社区、分享或更多微服务。
- 暂不继续增加 Agent 动作数量；先用真实评测确认现有提示、错因、相似题和主观题评分的质量瓶颈。
- 暂不迁移 Vite/Tailwind 或重写视觉体系；现有前端能构建，优先补行为测试和真实链路。

---
# 2026-09-12 进一步纠偏：移除用户体系，收敛为单实例单租户

## 对需求的准确理解

本轮不是继续做游客登录，也不是把账号功能做得更轻，而是：

- 从产品和运行时行为中移除登录、注册、OAuth、邮箱验证码、匿名 Cookie、用户隔离和跨用户权限判断。
- 每次部署只有一套数据库和一套题库，所有题目、解析、聊天、练习、标签、分类和 Agent proposal 都属于这个实例。
- 右上角 H 不再承担用户身份，恢复为普通品牌/设置入口或直接移除“用户”语义。
- 现有数据库里的数据必须保留；迁移以“先统一归并、验证、再清理”为原则，不直接删表或 reset。

## 数据迁移策略

不要在一次提交里直接 `DROP users` 或删除所有 `user_id` 字段。建议分两步：

1. 运行时先切到单租户：停止创建/读取 Cookie 和 Session，所有 repository/service 不再按 user 过滤；现有 `user_id` 列暂时保留为 legacy 字段并统一写入固定 owner（例如 `1`），这样旧数据库和旧对象仍能读取。
2. 编写幂等迁移脚本并先备份数据库：将所有用户数据归并到单实例；对同名分类/标签去重并重映射 `question_tags` 与 `category_id`；检查 proposal/job/chat/练习外键和唯一索引；对象存储从 `users/{userId}/questions/...` 迁移到单租户路径或保留兼容读取。
3. 新旧代码稳定运行一轮后，再单独提交可选 schema 清理（删除 `users`、`user_sessions` 和业务表 `user_id` 列）。如果清理会增加风险，保留 legacy 列也可以，关键是运行时不再暴露多用户概念。

## 新 Goal 指令

```text
请以“移除 ErroNotebook 用户体系并完成单实例单租户数据迁移”为本次 goal，实际修改代码、迁移数据库、测试和文档；不要只输出方案。项目是 GitHub 上供朋友自行部署的本地/私有应用，不是集中式 SaaS。

一、基线和不可破坏项
1. 先阅读 AGENTS.md、project.md、webdesign.md、talk.md 最新记录、README.md、backend/API.md、ai-service/README.md，以及 backend、ai-service、frontend 全部用户/会话相关实现和测试。
2. 先运行 Python 测试与评测、Go 测试/vet、前端测试/TypeScript/构建，记录基线。不得破坏导入、OCR、异步 Job、解析、作答、聊天、taxonomy、proposal、练习和学习状态功能。
3. 不执行 git reset/checkout，不删除 `backend/uploads` 或现有数据库数据。所有 schema 变化必须可重复执行，并提供备份/回滚说明。

二、移除运行时用户语义
1. 前端右上角 H 不再显示用户/游客/登录语义；改为品牌或设置入口。移除登录注册入口、游客提示、账号相关文案和 `/session` 业务依赖。
2. Go 不再创建或校验 `erro_session` Cookie，不再注入 `UserID` context，不再对 Question、Job、Analysis、ChatMessage、PracticeSession、AIProposal、Category、Tag 等做用户过滤或越权 404。所有请求直接访问当前单实例数据。
3. 删除或停用 auth middleware、`User/UserSession` 业务模型、`UserIDFromContext`、`currentUserID`、`GetForUser` 等运行时依赖；清理 handler、service、repository、storage key 和测试中的用户参数。Python 不增加任何认证逻辑。
4. API 文档和前端请求不再要求 `credentials`、userId、anonymous notice 或登录状态；保持现有业务路由和响应结构稳定，避免把单租户改造变成前端功能回退。

三、现有数据库安全迁移
1. 第一阶段采用兼容迁移：保留旧 `user_id` 列和旧表，新增/确认固定单实例 owner（如 `1`），把所有业务行的 `user_id` 归一到该 owner；运行时不再依赖这些字段。
2. 对 Category/Tag 按规范化名称去重：保留确定的一条记录，更新所有 QuestionTag 和 Question.CategoryID 引用，删除重复记录；系统分类和用户自建分类在单实例中都变成普通可编辑词表，不能丢失题目关联。
3. 检查并修复所有外键、唯一索引、proposal 的 idempotency key、Job/Chat/Practice 关联和分析记录；迁移必须支持重复执行且不产生重复关系。
4. 处理对象存储兼容：新文件使用 `questions/{questionId}/...` 或等价单租户 key；旧的 `users/{userId}/...` 文件要么在迁移中移动，要么实现旧 key fallback，不能因为移除用户字段导致图片题失效。
5. 提供明确的迁移命令/SQL 或 Go migration、执行前备份检查、迁移后行数与关联完整性校验；先保留 legacy 表/列一个版本，确认稳定后再决定是否物理删除。

四、保留基础运行安全，但不做账号系统
1. Python 仅内网可访问；保留文件 MIME/大小校验、请求体上限、超时、Worker 并发上限、失败重试和日志脱敏。
2. API Key 只来自本机 `.env` 或 Docker secrets，不进入用户表、浏览器 localStorage、URL 或日志。设置页只展示 provider/model/是否配置。
3. 如果保留限流，只按 IP/实例成本做简单保护，不引入 user/session 额度和账号配额。

五、功能和 Agent 继续作为主线
1. 把 p0/p1 评测改为逐例执行并校验 expected/forbidden，报告真实 provider、mock 和未验证项；补真实 408 样例的有限预算验收。
2. 完善三级提示、错因诊断的解题过程追问、换种讲法的卡点选择、聊天上下文和失败状态。
3. 修复主观题评分确认后的 `ungraded/manual_required` 状态语义，完善相似题答案/解析一致性和 proposal 过期校验。
4. 若引入 LangGraph，只在 Python 内实现有限步数的 `QuestionTutorGraph`：preflight → assess_evidence → choose_intervention → generate → validate → repair/needs_review/completed；Go 继续负责业务数据和最终持久化，禁止无限自主循环。

六、测试和完成条件
1. Go 测试覆盖：无 Cookie 也能访问、旧 Cookie 不影响结果、不同旧 user_id 的数据归并后都可见、分类/标签重映射、旧图片 key fallback、迁移重复执行、proposal/job/chat/practice 关联完整。
2. 前端测试覆盖：无登录状态正常工作、H 图标不再呈现用户语义、导入/轮询/追问/练习流程不回退。
3. Python 测试和 Agent 评测保持原有通过；真实 provider 质量单独报告。
4. 同步 AGENTS.md、README.md、project.md、webdesign.md、backend/API.md、ai-service/README.md 和 `.env.example`，删除“匿名用户/注册/跨用户隔离/账号升级”等过时产品文案。
5. 最终交付：变更清单、迁移脚本/SQL、备份和回滚说明、测试命令及结果、数据完整性校验结果、Agent 功能改进和剩余限制。只有运行时单租户化、旧数据可读、核心功能无回退且文档一致后才能宣布 goal 完成。
```

---

# 2026-09-12 方向纠偏：项目定位为自部署单租户应用，停止扩展登录体系

## 决策

本项目是发布到 GitHub 供朋友自行部署的完整前后端项目，不是由作者集中托管的 SaaS。每个部署实例拥有自己的 MySQL、Go、Python 服务和 Provider 配置，默认天然是单租户。因此：

- 登录、注册、OAuth、邮箱验证码、账号升级、跨用户隔离、跨设备同步不再属于当前产品主线。
- 之前关于“匿名升级正式账号”的 Goal 由本记录 supersede，不应继续按那个方向扩展功能。
- 现有匿名 Cookie/`UserSession` 代码暂时不要直接大规模删除，先作为当前实现的兼容层；后续可在稳定功能后评估是否简化为单实例 owner。不要为了删除它而破坏已有数据和主链路。
- 如果用户把实例暴露到公网，仍需做基础运行安全（内网限制、CORS、文件大小、请求超时、限流和密钥脱敏），但不需要做完整用户系统。

## Provider Key 的正确定位

- 自部署项目优先使用 `.env` / Docker secrets 配置 `OPENAI_API_KEY`、`VISION_API_KEY`、模型和 Base URL；API Key 属于这台机器的部署配置，不属于业务用户资料。
- 设置页可以展示当前 provider、模型和“是否已配置”，但不建议把 API Key 存到业务数据库或让前端负责保管。
- Docker Compose 应提供 `.env.example`、MySQL、Go、Python、前端的最小启动方式和健康检查；真实 Key 只通过本地 `.env` 或 secrets 注入，日志绝不打印。

## 新的下一阶段 Goal 指令

```text
请以“完成 ErroNotebook 自部署单租户版本的功能收口与 Agent 质量提升”为本次 goal，实际实现、测试、验收并同步文档。项目面向 GitHub 用户自行部署，不是集中式 SaaS；不要新增登录、注册、OAuth、邮箱验证码、账号迁移、跨用户隔离或跨设备同步功能。

一、确认部署边界
1. 先阅读 AGENTS.md、project.md、webdesign.md、talk.md 最新记录、README.md、backend/API.md、ai-service/README.md 和当前代码；运行 Python/Go/前端基线测试并记录结果。
2. 以单实例单租户为默认模型：一套部署对应一套 MySQL 和一套 Provider 配置。现有 User/UserSession 代码先保持兼容，不把本轮时间花在重写或删除认证表上。
3. API Key 只通过 Go/Python 服务端 `.env` 或 Docker secrets 注入；设置页只展示配置状态，不保存明文 Key，不放 localStorage、URL、数据库或日志。未配置真实 provider 时明确显示 mock/配置缺失，不能伪装真实效果。

二、Docker 与开箱即用
1. 增加或完善 `docker-compose.yml`、各服务 Dockerfile、`.env.example`、健康检查、启动依赖和数据卷；至少支持 MySQL + Go + Python AI service 的一键启动，前端提供开发/生产两种方式。
2. README 给出 Windows/macOS/Linux 的最短启动路径、端口、迁移、Provider 配置、日志查看、停止和数据备份说明；默认不要求注册账号。
3. Go 只对外暴露业务 API；Python AI 服务默认只在内部网络可访问。保留上传类型/大小校验、请求超时、并发上限、错误脱敏和基础限流，避免公网暴露时轻易拖垮实例。

三、优先打磨错题工作台和 Agent 功能
1. 把 p0/p1 评测改为逐例执行：每个案例都要真实构造 QuestionContext、调用动作并检查 expected/forbidden，报告逐例结果和聚合指标；mock 与真实 provider 分开统计。
2. 补 Agent 端到端闭环和前端测试：导入 → OCR → 解析 → 作答 → 错因诊断/提示/换种讲法 → 追问 → 保存；覆盖 needs_input、needs_review、failed、重试、取消和模型不可用。
3. 实现真正的三级提示：记录当前题已使用的提示级别，允许用户逐级请求，一级不泄露答案，二/三级逐步增加具体性；不允许每次都固定从一级开始。
4. 改进错因诊断：最终答案不足以判断概念混淆、条件遗漏、计算错误或方法错误时，先追问关键步骤，把证据与诊断对应展示；不确定时返回 needs_review。
5. 改进换种讲法和聊天上下文：允许用户指定概念、步骤或选项作为卡点；限制历史长度但保留最近目标和已解释内容，避免重复回答。
6. 改进相似题和主观题评分：增加独立答案/解析一致性校验、机械复制检测、评分依据展示、人工确认状态和过期指纹；修复确认后仍显示 `ungraded/manual_required` 的状态语义。
7. 如果引入 LangGraph，只实现一个有限步数的 `QuestionTutorGraph`：preflight → assess_evidence → choose_intervention → generate → validate → repair/needs_review/completed，并加入“提示 → 等待用户回答 → 判断是否掌握”的短循环。Go 仍负责业务数据和最终持久化，图不能访问 MySQL 或无限自主循环。

四、学习闭环和页面质量
1. 优先完善题目详情、解析、追问、练习结果、标签/分类和异步状态的交互，保持桌面三栏工作台，不新增独立 Agent 页面。
2. 补练习模式的主观题 AI 建议展示、确认后状态、学习状态更新边界和推荐原因；客观题继续由 Go 规则判分。
3. 增加真实浏览器级 smoke 或最小 Playwright 验收，验证上传、轮询、切题、追问、失败重试和生产构建，而不仅是组件渲染测试。

五、完成条件
1. Python 测试、Go 测试/vet、前端测试/TypeScript/构建、逐例 P0/P1 评测和 Docker 启动检查全部可复现。
2. 至少用有限预算验证真实 provider 的 408 样例；报告明确区分真实、mock、未验证项，不虚构 Agent 教学质量。
3. 同步 README、project.md、webdesign.md、backend/API.md、ai-service/README.md 和 `.env.example`；删除或改写所有“必须登录/账号升级/多用户 SaaS”式旧规划文案。
4. 最终交付能力清单、启动命令、关键接口、评测报告、已知限制和下一步建议。只有代码、测试、文档和自部署路径均完成后才能宣布 goal 完成。
```

---

# 2026-09-12 下一阶段 Goal：匿名优先账户、Provider 设置与 Agent 编排收口

下面这段可以直接作为下一次实现任务的 Goal 指令。目标是实际修改代码、测试和文档，不只输出方案；先检查并保留当前工作区修改，不执行 reset/checkout，不删除本地上传数据。

```text
请以“完成 ErroNotebook 匿名优先账户体系、AI Provider 设置和单题辅导 Agent 编排收口”为本次 goal，实际实现、测试、验收并同步文档。

一、先确认基线与范围
1. 完整阅读 AGENTS.md、project.md、webdesign.md、talk.md 最新记录、README.md、backend/API.md、ai-service/README.md，以及当前 Go/Python/前端实现和测试。
2. 先运行现有 Python 测试与 P0/P1 评测、Go 测试/vet、前端测试/TypeScript/构建，记录基线；不得回退匿名隔离、异步 Job、图片真实字节、taxonomy 自动应用、proposal 幂等和失败不伪装 completed 等已有约束。
3. 本轮核心范围是：游客默认使用、登录/注册入口、邮箱验证码、一个 OAuth/OIDC 提供商、游客数据升级迁移、Provider API Key 设置、限流/额度和必要安全测试。不要引入支付、复杂 RBAC、社交功能、知识图谱、向量库或新的业务微服务。

二、游客优先与登录注册体验
1. 将前端右上角 H 图标改为“登录 / 注册”入口；未登录时显示轻量“游客模式”状态，不弹窗阻断导入、OCR、解析、追问和练习。设置页说明：游客数据只绑定当前浏览器，清除 Cookie 或更换设备后无法恢复。
2. 登录入口提供邮箱一次性验证码（或 magic link）和一个 OAuth/OIDC 提供商（优先 GitHub 或 Google，选择一个即可）。前端只能调用 Go `/api/v1/auth/*`，不能直连 OAuth、邮件服务或 Python。
3. Go 负责 OAuth state/PKCE、回调、code/token 校验、邮箱验证码生成与校验、正式 Session、登出和当前用户信息；Python 不承担身份认证。OAuth client secret、邮件凭据只能在 Go 服务端环境变量或密钥管理中保存。
4. 邮箱验证码只保存哈希值，10 分钟过期、单次使用、最多 5 次尝试，并按 IP、邮箱和匿名 Session 限制请求频率；异常频率返回 429 或要求 CAPTCHA，不让正常朋友每次登录都遇到验证码。
5. 登录已有账号时不要自动合并另一个账号的数据。游客升级必须显式确认，并在事务中迁移当前匿名 User 的题目、附件、解析、聊天、练习、学习状态、taxonomy 和 proposal；迁移后轮换 Session。重复点击、超时和并发升级必须幂等。
6. 账号模型保持业务表的 `user_id` 归属不变；建议新增 `user_identities`（email/oauth provider + subject 唯一约束）和必要的 `auth_challenges`，保留 `users.kind=anonymous/registered`。增加账号注销/数据删除或至少预留清晰接口，不允许遗留孤立附件。

三、设置页 Provider API Key：可用，但必须服务端保护
1. 设置页新增 AI Provider 配置：provider、baseURL、model、API Key；游客也可以填写，配置归当前匿名 User/浏览器，不要求先注册。注册升级后可选择迁移配置，默认不自动跨账号复制。
2. 前端不把 API Key 放入 localStorage、URL、普通日志或页面回显；请求只走 Go。Go 使用 `APP_ENCRYPTION_KEY`（或等价密钥管理）加密存储，读取接口只返回 provider/model、是否已配置和 key 后四位，绝不返回原文。
3. Go 调用 Python 时只在内存中传递当前请求所需的 Provider 配置，Python 不落盘、不记录、不回传 API Key；内部接口增加服务间认证或仅允许内网访问。没有配置用户 Key 时，按现有服务端 `.env` provider fallback；两者都没有时返回明确 `AI_CONFIG_MISSING`，不能静默切到 mock。
4. 提供“测试连接”接口，但必须使用独立短超时、调用预算为 1、日志脱敏；成功只返回 provider/model 和耗时，不返回上游原始响应。删除/替换 Key 时清理旧密文，不能在错误信息里泄露 Key。
5. 为上传、OCR、解析、Agent、聊天分别设置用户/IP/Session 额度；限制请求体、图片字节/数量、并发 Job、模型调用次数、单用户存储量和总 deadline。达到额度返回可区分的 429/配额错误，并在前端提示如何稍后重试或注册。

四、LangGraph 只用于真实的单题辅导编排
1. 不要把每个现有动作机械包成节点。保留 Go 作为业务入口、权限和最终持久化；Python 侧将现有 AgentDispatcher 重构为有限步数的 `QuestionTutorGraph`，或在隔离模块中提供等价实现。
2. 图至少包含：`preflight`（缺作答/题面残缺）→ `assess_evidence` → `choose_intervention`（hint/diagnose/explain）→ `generate_structured_result` → `validate_domain` → `repair`（最多一次）/`needs_review`/`completed`。动作仍由 Go 显式指定，模型不能自行改变 action。
3. 增加一个可演示的短循环：第 1 级提示 → 等待用户回答（显式暂停）→ 判断是否理解 → 第 2/3 级提示或结束。最多 3 次干预、总 deadline、可取消、上下文和 token 上限，禁止无限自主循环。
4. LangGraph state 只保存受限 QuestionContext、当前节点和版本指纹；Go 继续保存聊天、作答、学习状态和 proposal。图不能访问 MySQL、发送任意网络请求、创建正式 Question 或修改成绩。
5. 记录 `traceId`、graphVersion、node、provider/model、duration、attempts、source(mock/real) 和最终状态，不能记录题面全文、图片 base64、完整作答或密钥。

五、测试、评测和文档完成条件
1. 增加 Go 路由/服务测试：游客可用、OTP 过期/重复/暴力尝试、OAuth state 失败、Session 轮换、游客升级迁移、重复升级、跨用户 404、限流/额度和 API Key 脱敏。
2. 增加 Python/Go 契约测试：用户 Provider 配置只在内存请求中传递；未配置真实 provider 不伪装 mock；LangGraph 的 needs_input、needs_review、failed、取消和最大步数均能收敛。
3. 前端增加登录/注册入口、游客提示、设置保存/测试连接、登出、升级后刷新和错误状态测试；不得在测试快照或 DOM 中出现完整 API Key。
4. 将 p0_cases.json、p1_cases.json 改为逐例执行并校验 expected/forbidden，补充账号安全、Key 脱敏、LangGraph 分支和三级提示案例；mock 与真实 provider 报告分开，不能把 mock 结果当作教学质量。
5. 同步 AGENTS.md（不再写“早期规划阶段”）、README.md、project.md、webdesign.md、backend/API.md、ai-service/README.md 和 `.env.example`，明确游客/注册、Provider 配置、OAuth/OTP、限流、LangGraph 边界和已知限制。
6. 最终交付必须包含：能力清单、数据库迁移、接口示例、密钥存储与脱敏说明、测试命令及结果、逐例评测报告、真实/mock 范围、未完成项和下一步建议。只有实现和必要验证均完成后才能宣布 goal 完成。

推荐默认决策：邮箱验证码 + 一个 OAuth 提供商；游客立即可用；AI Key 由 Go 加密保存；LangGraph 只编排单题辅导短循环；不增加独立 Agent 平台。
```

---

# 2026-09-12 登录注册策略与 LangGraph 适用边界讨论

## 一、登录/注册建议：匿名优先，注册作为“升级”而不是使用前置条件

当前已经有签名 HttpOnly `erro_session` Cookie 和匿名 `User/UserSession`，这个方向是对的。推荐分三步做：

1. **首次访问直接进入游客模式。** 不要求注册即可导入题目、OCR、解析和追问；页面明确提示“数据仅绑定当前浏览器，清除 Cookie 后无法恢复”。这样朋友可以零门槛体验。
2. **需要跨设备/长期保存时再注册。** 第一版账号建议采用邮箱 magic link/一次性验证码，或一个 OAuth 提供商，不要先做复杂密码体系。登录成功后，把当前匿名 User 的题目、附件、解析、聊天、练习和学习状态在事务中迁移到正式 User，随后轮换 Session Cookie。
3. **注册入口保持可见但不打断主流程。** 在题目数量、使用天数或换设备时提示“保存到账号”，而不是打开页面就强制登录。游客数据可设置保留期，正式账号数据不自动清理。

### 真正需要防护的是资源滥用，不是“账号数量”本身

大量注册只是表象，真正的攻击面是 AI 调用、图片存储、数据库连接和 Worker 队列。建议按成本分层限制：

- IP + Session + 账号三层限流；普通读接口较宽，图片上传、OCR、解析、Agent 动作分别设置更严的每分钟/每日额度。
- 新注册账号先完成邮箱验证；异常频率才触发 CAPTCHA、短暂冷却或人工解锁，不让正常朋友承担验证码成本。
- 限制请求体、图片数量/大小、单用户存储空间、并发 Job 数、单次模型调用次数和总超时；队列设置全局并发上限与熔断。
- 数据库加唯一索引、外键、分页上限和连接池上限；过期匿名 Session、孤立附件和失败 Job 定期清理。
- 记录 `user/session/IP/traceId/provider/duration/bytes` 等安全指标，但不记录密钥、图片 base64 或题面全文；对异常用户降速而不是直接删除数据。

因此，“注册方便”与“后端安全”并不矛盾。推荐的体验是：游客立即可用，注册只在需要恢复数据时出现，昂贵能力按额度保护。

### 建议的数据演进

保留现有业务表的 `user_id`，扩展：

- `users.kind`: `anonymous` / `registered`
- `user_identities`: `user_id`、`provider`、`subject/email` 唯一索引、验证时间
- `user_sessions`: 继续保存哈希 Token、过期时间和撤销时间
- `usage_counters`（或按日聚合表）：按用户/IP 记录上传字节、Job、模型调用和失败次数

匿名升级必须是事务：校验一次性凭据 → 锁定匿名 User → 合并/迁移资源 → 建立正式身份 → 轮换 Session。若邮箱已存在，不应静默合并两个用户的数据，而应要求用户明确选择登录已有账号或保留游客数据。

## 二、LangGraph 建议：可以用，但要用在“有状态的教学编排”上

不建议为了简历把当前每个动作简单包成一个 LangGraph 节点。现在的 `AgentDispatcher` 已经能完成显式动作路由、结构校验和有限修复，硬套图反而增加复杂度。

最适合本项目的落点是 **单题自适应辅导图**，仍由 Go 掌握业务数据和最终写库，Python/LangGraph 只做受限推理编排：

```text
接收 Go 构建的 QuestionContext
        ↓
preflight（缺少作答/题面残缺/题型不支持）
        ├─ needs_input → 返回需要用户补充什么
        └─ assess_evidence（答案、解析、错因证据、历史提示）
                ↓
        choose_intervention（hint / diagnose / explain）
                ↓
        generate_structured_result
                ↓
        validate_domain
          ├─ repair（最多一次）
          ├─ needs_review
          └─ completed
```

更能体现 LangGraph 价值的第二阶段是带用户反馈的短循环：

```text
评估当前理解 → 给第 1 级提示 → 等用户回答 → 判断是否掌握
       ├─ 已掌握 → 总结并结束
       ├─ 未掌握 → 第 2/3 级提示
       └─ 证据不足 → 追问关键步骤
```

这个循环必须有硬边界：最多 3 次干预、总 deadline、可取消、上下文长度上限；“等待用户回答”是显式暂停点，不允许模型无限自主循环。Go 仍负责保存聊天、作答、学习状态和 proposal，LangGraph 不直接访问 MySQL，也不直接创建题目或修改成绩。

### LangGraph 在简历上应该展示什么

重点不是“用了某个框架”，而是能说明：

- 为什么需要状态图：动作选择依赖题面完整度、答案证据和用户反馈；
- 如何做人机协同：`needs_input`、`needs_review` 和人工确认是图上的合法终态；
- 如何保证安全和成本：显式节点白名单、最大步数、超时、重试预算、结构化输出校验；
- 如何与业务系统解耦：Go 传入受限上下文，Python 返回结果，最终持久化由 Go 执行。

这比“增加一个自主 Agent 页面”更符合错题工作台，也更容易在面试中讲清楚工程取舍。

## 三、推荐执行顺序

1. 先完成游客限流、AI/存储额度和过期数据清理，暂不急着做完整注册页面。
2. 增加邮箱 magic link 或单一 OAuth 的匿名升级闭环，并补跨用户/重复升级测试。
3. 先把现有 Agent 评测和状态语义收口，再将 `AgentDispatcher` 内部重构为一个有限步数的 `QuestionTutorGraph`。
4. 最后再加入“等待用户回答 → 继续提示”的短循环，用真实评测数据验证它确实改善了学习结果。

---

# 2026-09-12 项目进度复核与 Agent 下一阶段完善建议

## 当前阶段判断

- 项目已经从规划期进入 **MVP 功能收口与真实质量验证阶段**。图片/手动导入、OCR 结构化、题目管理与作答、AI 解析、题目追问、错题沉淀、练习与学习状态的主链路已有代码实现。
- Go 业务入口、Python AI 服务和桌面三栏前端的边界基本符合既定架构；匿名会话隔离、异步 Job、taxonomy 自动落题、图片追问、相似题 proposal、主观题评分建议也已有实现。
- P0 四类动作和 P1 三类能力已经具备类型化协议与 mock/契约测试，但当前更准确的结论是“工程链路可运行”，不是“真实 Agent 教学质量已达标”。
- 工作区当前仍有一批未提交的工程清理和说明文档修改，应先形成一个可回退的干净提交，再开始下一轮 Agent 改造。

## 本轮复核结果

- Python：`python run_tests.py`，26 tests passed。
- P0 离线 runner：读取 20 条案例，四动作 mock smoke 通过。
- P1 离线 runner：读取 24 条案例，taxonomy / 相似题 / 主观题评分的少量 mock 契约检查通过。
- Go：`go test ./...`、`go vet ./...` 通过。
- 前端：3 suites / 4 tests 通过，TypeScript 检查与生产构建通过；仅有 React Router v7 future flag 和 Node `fs.F_OK` 既有警告。

## 关键结论：Agent 目前最需要补的不是更多动作，而是真实质量闭环

### P0：下一轮必须优先完成

1. **把离线案例变成真正可执行的评测集。** 当前 `p0_cases.json`、`p1_cases.json` 的 20/24 条记录主要被 runner 计数，脚本没有逐例构造上下文、调用动作并校验 `expected` / `forbidden`。需要逐例执行，输出每例通过原因、失败证据和聚合指标，不能再把“案例文件存在”当作“案例已评测”。
2. **建立真实 provider 小规模金标验收。** 先选 408 四科各 10～20 道、覆盖正常题和坏输入的人工核对集，分别测答案/解析一致性、错因证据、一级提示泄露率、taxonomy 大类准确性、相似题可解性、评分建议与 rubric 一致性。mock 与真实报告必须分开。
3. **补 Agent 端到端测试。** 当前前端仅 4 个测试，没有覆盖 Agent 按钮、`needs_input` / `needs_review` / `failed`、proposal 确认/拒绝、图片追问失败重试和评分确认；Go 也缺少 proposal、主观题评分、过期指纹、重复确认、跨用户访问等服务/路由级测试。
4. **修正主观题确认后的状态语义。** 当前人工确认分数后仍写 `GradingStatus=manual_required`、`Status=ungraded`，界面可能继续显示“待批改”。应明确 `ai_suggested`、`confirmed`、`manually_adjusted` 等状态，并决定确认后是否只记录分数，还是在明确规则下更新学习状态。
5. **收紧动作入口边界。** 通用题目 Agent 接口当前也允许 `grade_subjective_answer`，但评分确认只存在于练习会话路径，容易产生无法完成确认的孤立 proposal。评分动作应只从带 session/orderIndex/answer fingerprint 的练习流程发起。

### P1：提升“辅导效果”，而不是提升自主性

1. **让提示形成真实三级递进。** 前端现在每次固定请求 `hintLevel=1`；需要按题目和当前会话记录已用层级，允许“再给一点提示”，并对 1/2/3 级分别设定答案泄露规则。
2. **错因诊断先收集作答过程。** 只有最终答案时不应强判“概念混淆/条件遗漏”。可先追问用户思路或关键步骤，再区分知识缺口、审题、方法、计算和表达问题，并把证据片段与结论对应展示。
3. **换种讲法支持用户指定卡点。** 当前前端把 focus 固定为“当前不理解的概念或步骤”；应允许用户选择概念、某一步或某个选项，并根据已有对话避免重复同一种解释。
4. **增强相似题质量闸门。** 当前主要检查 schema、选项键、答案引用和简单的机械复制。还需要独立求解/规则复核、答案与解析一致性检查、与原题相似度上下限，以及确认前完整预览和可编辑能力。
5. **增加轻量学习者上下文。** 不需要引入自主 Agent 平台；只需在 Go 侧汇总最近错因、掌握度、已用提示和偏好讲法，作为有界上下文传给 Python，用于控制解释深度和推荐难度。

### P2：进入可部署阶段前补齐

- 为 Agent 动作增加真正的总 deadline、取消传播、并发/频率限制和明确的重试后状态；当前主要是单次调用 timeout，最坏耗时可能随尝试次数累加。
- 把 prompt 从代码字符串提取为可版本化模板，记录模型、promptVersion、输入版本、耗时、token 和人工采纳/驳回反馈，支持回归对比。
- 增加真实运行监控：成功率、`needs_input`/`needs_review` 比例、provider 超时、降级率、提示泄露告警、proposal 采纳率和人工改分幅度。

## 文档与 Agent 协作约定需要同步更新

- `AGENTS.md` 仍把仓库描述为“早期规划阶段”，已经落后于当前 MVP 收口状态；应改为“实现已存在，修改前先跑基线并保护数据/迁移兼容”。
- `README.md`、`ai-service/README.md`、`project.md`、`backend/API.md` 和评测案例之间存在口径漂移：例如 taxonomy 已改为自动应用和允许创建用户私有知识点标签，但部分说明/案例仍写“等待用户确认、不得创建标签”；主观题评分也仍有“后续扩展”的旧文案。
- 后续要求 Agent 每次改动作契约时同时更新：Python schema、Go client/API、前端类型与状态、可执行评测案例、报告和顶层 README；其中任一项未同步，不视为完成。

## 推荐执行顺序

1. 整理并提交当前未提交改动，建立干净基线。
2. 修正文档和评测案例的 taxonomy/评分状态口径。
3. 把 44 条离线案例改造成逐例执行的评测 harness。
4. 补 Go/前端 P0 端到端测试，并修复评分确认状态。
5. 在有限预算下跑真实 408 金标集，形成第一份真实质量报告。
6. 只有报告暴露出明确问题后，再迭代提示词、模型和三级提示/错因追问交互。

---

# 2026-09-10 前端标签页图标切换为 Husky 小狗 Logo

## 本轮完成

- 浏览器标签页 favicon、Apple touch icon 和 PWA Manifest 统一使用现有 `line-husky-nobackground.png`。
- 删除不再引用的 CRA 默认 `favicon.ico`、`logo192.png`、`logo512.png`，不改变页面功能。
- 运行前端测试、TypeScript 检查、ESLint 和生产构建确认图标资源可正常打包。

---

# 2026-09-10 第二轮项目清理：遗留资源、模板文档与评测脚本隔离

## 本轮完成

- 审计整个仓库的测试、评测和调试入口：正式 `*_test`、P0/P1 离线评测 runner 仍被当前代码或文档使用，继续保留；没有发现可安全删除的历史测试脚本。
- 修正 `evals/run_p0_eval.py`、`evals/run_p1_eval.py` 的离线边界，未显式设置时强制使用 mock，避免本地 `.env` 触发真实 provider 请求；同时统一 P1 runner 的异步结构。
- 删除未被引用的前端 CRA 遗留资源：`frontend/asset/` 下两张 Husky 图片、`frontend/src/logo.svg` 和无实际回调的 `reportWebVitals.ts`。
- 修正前端测试的 Testing Library DOM 访问规范；清理 CRA 默认 README、HTML、Manifest 文案，并更新 `CLAUDE.md` 的过时状态描述。
- 删除 `.gitignore` 中重复的 `**/__pycache__/` 规则；不触碰 `backend/uploads/` 本地用户上传数据。

## 验收

- Python：`python run_tests.py` 26 tests passed；P0 20 例、P1 24 例离线评测通过；Ruff 通过。
- Go：`GOCACHE=../.gocache go test ./...`、`go vet ./...` 通过。
- 前端：4 tests passed、TypeScript、ESLint、生产构建均通过；仅保留 React Router 既有 future flag 提示。

---

# 2026-09-10 项目工程清理：Python 静态规范与离线测试稳定性

## 本轮完成

- 清理 Python 未使用导入、无意义 `f` 前缀、单行条件语句和 lambda 风格问题；`ruff check app tests run_tests.py` 已通过。
- mock 聊天延迟改为读取既有的 `LLM_MOCK_DELAY_SECONDS` 配置，不再硬编码 1 秒；生产 provider 分支和响应契约不变。
- 增加 `ai-service/run_tests.py`，在未显式指定时默认使用 mock provider，避免本地 `.env` 让离线单测访问真实服务；同步更新根 README 和 AI 服务 README。
- 保留现有业务边界、API、数据模型和前端行为，没有删除功能代码或生成物。

## 验收

- Python：`python run_tests.py`，26 tests passed；`python -m pytest -q`，26 passed。
- Go：`go test ./...`、`go vet ./...` 通过。
- 前端：`npm test -- --watchAll=false`、`npx tsc --noEmit` 通过；React Router 仍有既有升级提示，但不影响构建或测试。

---

# 2026-09-10 taxonomy 结果默认落题，标签输入取消候选菜单

## 本轮调整

- 中栏不再显示“学科暂未确定”一类 taxonomy 文案；AI 未给出大类时，taxonomy 结果不在中栏打扰用户。
- AI 生成的标签不再依赖分类是否命中：Go 会将最多 3 个标签创建为当前匿名用户私有 Tag，并直接绑定到题目；标签会在右侧题目详情显示。
- 右侧标签输入框取消候选下拉菜单，输入标签后按 Enter 添加；历史标签仍可通过输入完整名称复用。

---

# 2026-09-10 改为 AI 自动落地分类标签，取消 taxonomy 确认流

## 本轮调整

- 根据用户反馈，taxonomy 不再展示“待确认 / 待应用”，也不再要求点击“应用建议”或“重新生成建议”。分析完成后 Go 自动写入 1 个 AI 选定的大类和最多 3 个标签；用户只在题目详情发现问题时手动修改。
- 分类仍限定为 408 顶层学科，标签用于同步互斥、生成树、进程调度等细知识点；AI 无法确定大类时才保持未分类并提示原因。
- 相似题保存、主观题评分等本身涉及新题入库或最终成绩的动作仍保留确认边界，不与题目 taxonomy 自动落地混用。

---

# 2026-09-10 408 大类自动分类与 OCR 考试界面噪声清洗

## 本轮完成

- 数据库迁移确保 `数据结构`、`计算机组成原理`、`操作系统`、`计算机网络` 四个 408 顶层系统分类存在；AI 自动分类只使用这些大类和当前用户自建的顶层学科，不再把“图论、TCP、进程调度”等细知识点当分类。
- AI 可返回最多 5 个细粒度知识点标签；Go 校验大类后，将尚不存在的标签创建为当前匿名用户私有 Tag，再对未人工设置 taxonomy 的题目默认应用。人工修改仍具有优先级，不会被后续分析覆盖。
- OCR 结构化新增考试界面前缀清洗，可移除倒计时、题目进度、题型、分值、难度和“第 N 题”；原始 OCR 保留在 `rawText`，清洗后写入 `ocr_exam_ui_noise_removed`。LLM 二次结构校验后再次清洗，防止噪声回流。
- 历史识别错误可在右侧详情点击“编辑”，修改题干后保存；新导入题目自动使用清洗后的题干。

## 验收

- 用户给出的 `倒计时00:17:35 24/31单选题（分值3.0分，难度：易） 下列哪一种图不一定是树（）。` 已有精确单测，输出题干为 `下列哪一种图不一定是树（）。`。
- 运行态发现并修复“无 warning 时 Go 把 `question.warnings` 发成 `null`、Python 因要求数组而返回 400”的契约问题；现在固定发送 `[]`，干净题面也能进入分析与自动分类。
- 真实 provider 验收：“进程和线程区别”自动分类为 `操作系统`，生成 `进程 / 线程 / 进程与线程区别`；用户示例“下列哪一种图不一定是树（）。”自动分类为 `数据结构`，生成 `连通图 / 无环图 / 图的边数`。后者因没有选项和可确认答案进入 `needs_review`，但 taxonomy 已自动落库；验收题已删除。
- 另一匿名会话只能看到系统标签，无法看到上述私有标签 `进程与线程区别`；四个 408 系统大类已通过 Go API 返回。AI 健康检查为 `ocrBackend=auto / llmBackend=openai_compatible`。
- Python 全量测试 `26 passed`；P0 `20` 例、P1 `24` 例通过；Go 全量测试与 vet 通过；前端 2 suites / 3 tests 及生产构建通过。

---

# 2026-09-10 图片解析变成 mock 的链路排查与修复

## 根因

- 运行中的 `ai-service` 健康检查曾返回 `{"llmBackend":"mock"}`，且本地 `ai-service/.env` 明确配置了 `LLM_BACKEND=mock`；页面显示的“当前为 mock 解析结果……”正是 `_mock_analysis` 的固定摘要，不是 OCR 或前端轮询生成的内容。
- 切换到真实模式后发现沙箱内无法连接 `api.deepseek.com` / `api.siliconflow.cn`，沙箱外 443 连通；视觉 provider 不可达时原逻辑会直接 504，且 Go 只有 `HasDiagram=true` 才把原图传给 AI，普通图片题会丢失视觉输入。

## 修复

- 本机 `.env` 改为 `LLM_BACKEND=auto` 并重启 AI 服务；健康检查恢复为 `ocrBackend=auto / llmBackend=openai_compatible`，不再静默走 mock。
- Go 分析任务只要存在服务端管理的 `ImagePath` 就转发原图，不再用 OCR 推断的 `HasDiagram` 决定是否发送图片。
- Python 对视觉 provider 增加动作级超时；视觉 provider 暂时不可用且文本 provider 可用时，基于 OCR 结构化结果降级到真实文本 LLM，并返回 `multimodal_fallback_to_ocr` warning；文本 provider 也不可用时明确失败并进入 Go 重试/失败状态，不生成伪成功 mock。

## 验收

- `GET /internal/v1/health`：`llmBackend=openai_compatible`。
- 真实文本调试请求返回非 mock 摘要；真实图片 multipart 请求在视觉不可达时返回真实文本降级结果和 `multimodal_fallback_to_ocr`，未出现 mock 固定摘要。
- 新图片 Go 链路已观察到 `OCR completed → analysis processing → analysis 200`；图片题不再因视觉 provider 失败而无限等待。
- Python 测试在 mock 隔离环境下 `20 passed`；真实 provider 联调受外部端点可用性影响，当前通过明确 warning/重试/超时暴露，不把外部不可达伪装成成功。

---

# 2026-09-10 分类创建、刷新默认选择与 AI taxonomy 默认应用修复

## 本轮完成

- 分类/标签改为“系统词表 + 用户私有词表”：匿名用户可以新增、修改、删除自己的分类和标签，系统项保持只读；因此题库中新建“计算机组成原理”不再被匿名只读策略拦截。
- 题库首次加载、刷新和筛选不会自动打开第一道题；“全部题目”和“未分类”默认收起，只有用户主动点击题目或带 `questionId` 路由时才打开。
- Go 将当前用户可见的分类/标签候选传给 Python；分析完成后，若题目尚未人工设置 taxonomy，Go 自动校验并应用 AI 的 category/tag，右栏标记“已自动应用，可手动调整”。人工分类/标签不会被后续分析覆盖。
- 没有候选时仍保留明确原因；Python mock 文案同步为“未人工设置时由 Go 默认应用，可手动调整”。

## 验收

- 本地匿名会话联调：创建分类返回 `201`；新图片任务从 `queued/queued` 进入 OCR、AI 阶段，完成后无需刷新即可读取题干、解析和自动应用的分类/标签；手动再次应用已自动落库的建议返回过期/幂等保护，不覆盖人工结果。
- 前端生产构建、前端测试、Go 测试/vet、Python 测试及 P0/P1 离线评测均已执行并通过；真实 provider 质量仍不由 mock 联调结论代替。

---

# 2026-09-10 匿名用户隔离基础与图片导入异步状态修复

## 本轮完成

- Go 新增最小 `User/UserSession`、签名 HttpOnly `erro_session` Cookie、首次访问自动创建匿名身份，以及当前会话归属解析；前端所有请求携带 Cookie，不信任传入 `userId`。
- Question、附件、分析、Job、聊天、批次、练习、学习状态和 AIProposal 均按用户隔离；题目与练习资源越权统一返回 404；Category/Tag 采用全局只读词表策略。
- 图片导入立即返回 `jobId/questionId` 和 `queued` 占位；worker 统一支持 `queued/processing/completed/needs_review/failed`，用 `processingStage` 区分 OCR 与 AI，并修复无任务时重复插入空 Job 的问题。
- 前端增加可取消、按 job/question 去重、1～2 秒起步、网络退避、5 分钟超时的轮询；上传占位、OCR 回填、解析中、待复核、失败重试和切题取消均已接入。

## 验收

- `python -m pytest -q`：19/19；P0/P1 离线评测：20/24；Go `go test ./...`、`go vet ./...`：通过；前端测试 2 suites/3 tests、生产构建：通过。
- 本地 MySQL/Go/Python mock 联调：图片任务从 `queued/queued` 自动变为 `completed/needs_review`，题干长度 851，分析接口 200，无需刷新；匿名会话复用、不同/伪造 Cookie 隔离通过。
- 另一匿名会话访问题目、聊天、解析、学习状态、proposal、Job、练习会话均返回 404，列表不包含对方题目；manual/practice 验收临时题已删除。
- 已知限制：真实 provider 质量、跨设备恢复、正式账号升级和 WebSocket 不在本轮范围；本地数据库中此前一次图片 smoke 的匿名测试记录未能通过原 Cookie 回收，未覆盖用户原有数据，后续可按该匿名会话清理。

---

# 2026-09-09 taxonomy 闭环与 P1 单题能力实现记录

## 本轮完成

- 标准解析由 Go 注入现有顶层分类/标签候选，Python 复用同一 taxonomy 校验并返回受限建议；建议保存指纹和候选快照，只有用户确认才应用，已有人工 taxonomy 不被覆盖。
- 图片追问通过 Go 服务端对象存储读取实际字节，以 Python multipart `files` 进入多图视觉 provider；文本请求保持兼容，失败不写伪成功 assistant 消息。
- 增加 `generate_similar_question` 和 `grade_subjective_answer` 类型化 action。相似题持久化为 `AIProposal`，确认/拒绝/过期/幂等由 Go 控制；主观题评分建议绑定作答指纹，人工确认后才写练习评分。
- 前端中栏增加相似题入口、候选确认/放弃、taxonomy 重新生成；练习结果增加 AI 建议分数和人工确认入口。

## 验证范围

- `python -m pytest -q`：19/19 通过（含 P0 基线与 P1 action/图片字节契约测试）。
- `python evals/run_p0_eval.py`：20 例 mock/契约通过；`python evals/run_p1_eval.py`：24 例离线 mock 规则检查。
- `GOCACHE=.gocache go test ./...`、`go vet ./...`：通过；默认 Go cache trim 受本机权限限制，因此验收使用仓库内独立 cache。`npm test -- --watchAll=false --runInBand`：2 suites / 3 tests 通过；`npm run build`：通过。
- 2026-09-10 本地 MySQL/Go smoke：首次访问、同 Cookie 复用、不同 Cookie 隔离、伪造 Cookie 重新建匿名身份通过；临时题目在另一匿名会话下的题目、附件关联入口、聊天、解析、学习状态、proposal、Job、练习会话均返回 404，列表不包含对方题目；manual/practice 验收临时题已删除。图片上传返回 `jobId/questionId` 与 `queued` 占位，worker 正常领取 OCR→AI 阶段任务，未再出现空 Job 重复插入。
- 真实异步联调补充：连接本地 Python mock（`ocrBackend=auto`、`llmBackend=mock`）后，图片任务从 `queued/queued` 变为 `completed/needs_review`，题干长度 851，分析接口 200；说明 OCR、AI 分阶段回填和待复核路径均已跑通，结果不代表真实 provider 质量。
- Python：`python -m pytest -q` 19/19；P0 离线评测 20 例、P1 离线评测 24 例均通过，仍属于 mock/契约验证，不代表真实 provider 质量。
- 未配置授权真实 provider 或 MySQL 时，真实视觉、真实模型质量和数据库端到端未验证，不把 mock 结果当成真实效果。

---

# 2026-09-09 当前版本提交信息与下一阶段 Goal

## 推荐提交信息

`feat: 完成单题辅导 Agent P0 升级与工作台接入`

建议提交正文：

- 增加可靠模型调用、统一错误和结构化输出校验。
- 实现错因诊断、换种讲法、分级提示、taxonomy 建议四类动作。
- 接通 Go 动作代理、学习状态保存、分类建议确认和中栏交互。
- 增加 Python/Go 契约测试、20 例离线评测及相关文档。

## 下一阶段决策

- 先修复标准解析未传 taxonomy 候选导致分类建议始终为空的问题，实现“自动生成建议、用户确认后生效”，继续禁止 Python 写业务库或自动创建分类标签。
- P1 依次实现真实图片追问、单道相似题候选、主观题辅助批改；三项都必须有类型化结果、版本/指纹校验、幂等确认、失败不伪装成功和真实/mock 区分。
- 图片追问必须传实际图片字节或安全内部 URL；相似题确认前不进入正式题库；主观题 AI 结果只是评分建议，最终分数和学习状态仍由 Go 控制。
- 完整可执行指令见 `ai-service/AGENT_GOAL.md` 第 5 节。本轮只生成下一阶段 Goal，没有启动 P1 实现。

---

# 2026-09-07 ai-service P0 单题辅导 Agent 实施与验收

## 本轮已完成

- 按 `ai-service/AGENT_GOAL.md` 第 3 节执行 P0；保留工作区原有内容，未执行 reset/clean。
- Python 增加可靠调用基础：async 调用通过线程隔离阻塞 urllib，真实/mock 明确区分，标准错误包含 code/message/retryable/traceId；动作调用最多 3 次，结构修复最多 1 次。
- Python 增加显式 `AgentDispatcher` 与 `/internal/v1/agent/actions`，完成 `diagnose_mistake`、`explain_alternative`、`hint`、`suggest_taxonomy` 四类动作及上下文、历史 role、候选 taxonomy 和题面质量约束。
- Go 增加动作类型/client，`归纳错因` 调用诊断并按证据保存；保存前做题目内容指纹校验，不改变掌握度、正确率、错题次数和复习日期。
- Go 增加 `POST /api/v1/questions/{id}/agent-actions` 与 `POST /api/v1/questions/{id}/taxonomy-suggestion/apply`；分类/标签确认应用使用事务、现有候选和合法性校验，不自动创建 taxonomy。
- 三栏工作台接入“换种讲法”“分级提示”“归纳错因”和“确认应用分类建议”；相似题生成等 P1 能力未实施。
- 新增 20 例离线评测样例、Agent 契约/故障测试和评测脚本；README、backend API、project/webdesign 需保持与本记录一致。

## 验证结论

- Python 基线及新增测试通过后，以 `python -m pytest -q` 为准记录结果。
- Go 测试业务包曾通过，但当前环境 Go build cache 位于 `C:\Users\Husky\AppData\Local\go-build`，裁剪/写入时报 `Access is denied`；需在可写 Go cache 环境复跑完整 `go test ./...`。
- 前端应执行 `npm run build`；真实 provider、数据库和真实端到端动作未配置时，只能认定 mock/契约链路，不能宣称真实模型质量验收完成。

---

# 2026-09-07 ai-service Agent 能力提升计划与 Goal 指令

## 本轮范围

- 用户希望规划下一步，重点完善 ai-service 提供的 Agent 功能，并获得具体可执行的 goal 指令。
- 本轮只编写计划与指令，未启动 goal、未修改业务实现、未运行模型调用或业务测试。
- 已结合当前工作区的 Python 服务、Go 归纳错因/追问实现及产品文档检查现状；现有未提交修改全部保留。

## 推荐决策（待后续实现）

- 优先把当前 Go 规则拼接的“归纳错因”升级为 Python 基于题面、作答、解析与追问证据生成的结构化诊断，由 Go 校验和保存。
- 本次建议的 P0 顺序：可靠调用和错误处理 → 单题上下文与四类动作（错因诊断、换种讲法、分级提示、分类标签建议）→ Go 与中栏最小接入 → 离线评测和真实条件下的验收。
- 保持 Question 为核心、Go 为唯一业务入口、Python 不写主业务库；Agent 使用轻量动作分发和有界步骤，不额外拆服务。
- 明确处理当前缺口：追问失败仍返回 completed、async 内阻塞网络调用、输出修复与语义校验不足、分类建议缺少候选约束，以及分析接口文档与 multipart 路由不一致。
- P1 后续依次考虑真实图片追问、相似题候选生成、主观题辅助批改，不纳入当前建议 Goal 的验收范围。

## 交付与下一步

- 完整计划及可复制指令见 [ai-service/AGENT_GOAL.md](ai-service/AGENT_GOAL.md)，包含边界、实施顺序、接口状态、四类动作输出、Go 保存规则、测试与评测标准。
- 后续可直接要求：“请将 ai-service/AGENT_GOAL.md 第 3 节完整指令设为本次 goal，并按其中 P0 范围实施、测试和验收；先检查当前工作区，保留已有修改。P1 留待后续。”
- 当前结论来自代码阅读，属于建议方案，不代表新能力已经实现或真实模型效果已经验证。

---

# 2026-09-04 首页详情、分类标签与三类页面边界调整

## 本轮产品决策

- 首页右侧详情只保留题目事实、OCR/解析状态、分类标签、答案、解析和 OCR 原文；移除“学习状态”区块及其中的掌握度、错因、薄弱标签、复习建议编辑交互。
- 学习状态字段、接口和中栏“归纳错因”动作继续保留。错因与掌握度属于跨题复习反馈，不在首页详情重复展示。
- `category` 是每题最多一个的稳定顶层学科分类；`tags` 是每题可多个的细粒度知识点标签。当前新建/更新分类不接受 `parentId`，历史 `parent_id` 字段保留兼容；TCP、UDP、HTTP、拥塞控制等只能作为标签。
- 题目分类更新支持显式传 `null` 清除分类。Go 校验 category 必须存在且为顶层分类；Go 对题目标签去重并校验 tag ID，前端不直连 Python。
- AI 分类准备采用解析结果中的可选 `taxonomySuggestion`：Python 只返回 `categoryName`、`tagNames` 和可选置信度，Go 保存建议并负责校验；本轮不自动创建分类或自动让建议生效。
- 做题模式负责“现在做什么”，学习统计负责“整体变化与洞察”，错题归档负责“题目资产检索与沉淀”。统计页不再列出归档式低掌握度题目，归档页不再承担薄弱标签/掌握度报表。

## 本轮已执行

- 前端首页详情移除学习状态展示与保存状态依赖，保留中栏 AI 归纳错因入口。
- 分类选择器改为“学科分类（单选）”，标签编辑器改为“知识点标签（可多选）”，增加职责提示。
- 统计页改为练习正确率、近期趋势、掌握度分布、薄弱学科和基于已保存 AI 归纳的洞察；增加去做题/查看归档入口。
- 归档页增加全文、学科、知识点标签、收藏、待校准和复习记录相关检索，列表仍回到单题工作台。
- 练习页强化推荐练习和作答后回到单题 AI 解析/追问的路径，并在选择题列表显示已有知识点标签。
- Go 增加分类/标签校验和 `categoryId: null` 清除分类支持；AI 分析契约增加可选的 `taxonomySuggestion` 字段。

## 后续计划（本轮不虚构已存在能力）

1. 在真实 AI 服务稳定后，补充基于题目与候选 taxonomy 的分类/标签建议生成，并增加“确认应用建议”的 Go 业务动作；应用动作必须复用现有分类单值与标签多值校验。
2. 若统计数据量增长，再将统计计算下沉为 Go 聚合接口；当前先复用已有题目、练习会话和推荐接口，避免新增微服务。
3. 继续做桌面端真实数据验收，重点检查空状态、长题干、分类删除、标签非法 ID、练习提交和 AI 解析失败恢复。

---

# 2026-08-24 简历项目描述建议

## 推荐项目名称

ErroNotebook｜AI 错题复习工作台

## 推荐简历描述（可直接使用）

面向考研 408 与理工科刷题场景，设计并实现以题目为核心的 AI 错题复习工作台，打通图片导入、OCR 结构化、人工校准、作答、AI 解析、上下文追问、错因归纳与复习推荐闭环。

- 采用 React + TypeScript 构建桌面端深色三栏工作台，分别承载题库组织、题目对话和题目详情/解析，支持题目搜索、筛选、标签、状态管理及多种题型作答。
- 采用 Go + Gin + MySQL/Redis 作为核心业务端，统一负责用户、题目、作答、学习状态、任务状态和对外 API；采用 Python + FastAPI 封装 OCR、题目结构化和 LLM 能力，前端不直接访问 AI 服务。
- 设计异步导入任务与状态机，覆盖上传、OCR、解析、完成、失败和待校准等状态；对低置信度、选项缺失、非法 JSON、超时和批量导入部分失败提供重试或人工编辑路径，避免任务永久卡在处理中。
- 将 OCR 原始文本、结构化题目、解析结果、用户作答和对话记录围绕 Question 实体沉淀；实现错因、薄弱标签、掌握度、错题次数、连续答对和下次复习时间的学习状态更新，并提供五类规则化练习推荐。
- 完成前端构建与测试、Go 单元测试、Python 测试及 Go → Python API 联调，验证单题/批量导入、人工校准、重新解析、题目追问、错因归纳、练习提交和推荐接口等核心链路。

## 面试时的项目定位

不要说“做了一个 AI 聊天机器人”，应表述为：这是一个围绕 Question 实体组织数据、由 Go 编排业务流程、由 Python 提供 AI 能力的单题深度处理与复习系统。

## 使用原则

- 没有真实数据时，不写用户数、准确率、响应提升百分比等量化指标。
- 如果后续补充了可验证数据，可把最后一条改成“覆盖 X 类题型、完成 X 个测试用例、平均响应耗时 X 秒”等结果型表述。
- 简历篇幅有限时保留前四条；面试版再补充状态机、失败恢复和学习状态设计。

# 项目下一阶段行动规划

更新时间：2026-08-19

## 一、当前判断

ErroNotebook 已经具备“图片导入 → OCR 结构化 → 题目展示 → 作答 → AI 解析 → 追问 → 错因归纳 → 复习推荐”的核心雏形，仓库中也已经有工作台、做题、统计、归档和设置页面，以及 Go 业务端、Python AI 服务和 MySQL 数据模型。

下一阶段的目标不是继续堆叠页面，而是把现有能力收敛成一个稳定、可重复演示、可以长期使用的 AI 错题复习工作台。

当前产品边界继续保持：

- 核心实体是 `Question`，不是聊天会话。
- 前端只访问 Go API，不能直接访问 Python AI 服务。
- Go 负责用户、题目、作答、学习状态、分类、鉴权和对外 API。
- Python 只负责 OCR、题目结构化、解析、追问等 AI 能力，并返回结构化 JSON。
- MVP 继续以图片题目为主，不恢复 PDF 试卷拆题、批次校对和整卷作答主线。
- 前端继续保持桌面端深色三栏工作台，不改成纯聊天产品。

## 二、阶段目标

### 目标 1：完成一次真实端到端验收

用真实题目验证整条链路，而不是只验证单个接口。

验收范围：

1. 图片上传和批量导入。
2. OCR 识别与题干、选项结构化。
3. 待校准题目的人工修正。
4. 题目详情、作答和基础判分。
5. AI 标准解析。
6. 围绕当前题的多轮追问和图片附件。
7. 错因归纳、薄弱标签和复习建议落库。
8. 推荐练习、练习提交和学习状态更新。
9. 统计页、归档页与题库数据保持一致。

建议准备 10 到 20 道真实样题，至少覆盖：

- 单选、多选、判断、填空。
- 简答、论述、计算。
- 操作系统、数据结构、计算机网络、组成原理。
- 清晰图片、低清图片、选项缺失或排版复杂的图片。

每道样题记录：导入是否成功、结构化是否正确、是否需要人工校准、解析是否成功、作答判定是否合理、推荐是否符合复习需求。

### 目标 2：优先修复影响使用的稳定性问题

重点不是增加功能，而是让用户在失败后仍能继续操作。

必须覆盖以下异常状态：

- 图片格式不支持、文件过大、上传失败。
- OCR 超时、OCR 返回空结果、结构化选项缺失。
- LLM 不可用、返回非法 JSON、解析超时。
- 追问失败或附件上传失败。
- 批量导入部分成功、部分失败、轮询超时。
- 题目保存失败、分类删除失败、网络中断。
- 推荐结果为空、主观题无法自动判分。
- 登录失效或无权访问其他用户的题目。

每种失败都要有明确状态、错误提示和下一步动作，至少提供“重试”“手动编辑”“返回题库”中的一种。禁止页面永久停留在“处理中”。

### 目标 3：补齐可持续复习所需的最小功能

以下功能属于当前产品闭环，应该优先稳定：

#### P0：必须完成

- 图片单题导入。
- 图片批量导入及逐项进度。
- OCR 结果展示和人工校准。
- 题目 CRUD、搜索、筛选、分类和标签。
- 单题作答和客观题基础判分。
- AI 标准解析。
- 题目上下文追问。
- 解析失败后的重新解析。
- 错因归纳并保存到学习状态。
- 练习记录、掌握度、错题次数、连续答对和下次复习时间。
- 今日复习、最近错题、薄弱专项、新题巩固、随机混合推荐。
- 工作台、做题、统计、归档页面的数据联动。

#### P1：完成 P0 后再做

- 真实用户登录、注册、密码哈希和用户数据隔离。
- 题目收藏、归档和批量操作。
- 解析结果版本记录，支持查看最近一次解析时间和来源。
- 复习计划的暂停、恢复和手动调整。
- 主观题人工批改及批改备注。
- 相似题生成，并允许用户确认后保存为新题。
- 统一的任务状态查询和重试机制。
- 关键操作日志和 AI 调用 traceId。

#### P2：暂缓

- 知识点图谱。
- 社区、分享、班级和复杂权限。
- PDF 试卷拆题。
- WebSocket 实时推送。
- 多端同步和复杂数据分析。
- 过度拆分的微服务。

## 三、推荐实施顺序

### 第 1 步：建立验收基线

- 固定本地启动方式和环境变量模板。
- 确认 MySQL、Go、Python、前端的启动顺序。
- 准备演示题目和最小数据库初始化数据。
- 记录前端 build、Go test、Python test 的执行命令。
- 建立一份端到端问题清单，按阻塞程度排序。

完成标准：新环境可以按照 README 启动，并能导入至少一道图片题。

### 第 2 步：修复导入和 OCR 质量

- 验证单图和多图导入的终态。
- 明确上传中、OCR 中、解析中、完成、失败、待校准状态。
- 检查题干、选项、题型、答案的结构化结果。
- 对缺选项、低置信度和 LLM 校验失败保留原始 OCR 文本。
- 确保人工校准保存后，题库列表、详情区和后续解析使用同一份数据。

完成标准：复杂图片识别错误时，用户能看见问题并手动修正，不会丢题或卡死。

### 第 3 步：稳定作答、解析和追问

- 为不同题型确认正确的输入控件。
- 客观题提交后显示答对、答错和正确答案。
- 主观题显示待批改，不自动伪造掌握度。
- 标准解析与追问必须绑定当前题目。
- 追问失败可以重试，历史消息按题目隔离。
- 解析和追问请求统一经过 Go 编排，并设置超时和错误处理。

完成标准：用户可以围绕一道题完成“先作答、看解析、继续追问”的完整体验。

### 第 4 步：稳定错因和复习闭环

- 验证错因归纳是否真正写入学习状态。
- 检查掌握度更新规则是否符合答题结果。
- 检查下次复习时间是否可解释。
- 验证五类推荐的数量、排序和推荐理由。
- 练习结束后，统计页和归档页立即反映最新结果。

完成标准：用户完成一次练习后，系统能告诉用户为什么错、应该复习什么、下一次什么时候复习。

### 第 5 步：做交付和展示收口

- 补齐 README 的快速启动、功能范围、架构图和演示流程。
- 准备一批稳定 demo 数据。
- 清理构建产物、本地配置和敏感信息。
- 确保前端目录可以被完整 clone，避免前端以不可用的子仓库形式存在。
- 固定一键启动或开发脚本。
- 做一次桌面端 UI 检查，重点关注长题干、长解析、空状态和失败状态。

完成标准：项目可以被他人按照说明启动，并在 3 到 5 分钟内展示完整核心闭环。

## 四、建议的接口和数据验收重点

### 题目

题目详情至少要能稳定返回：

- 题干、题型、选项、正确答案。
- 原始 OCR 文本。
- 结构化警告、结构化置信度和解析来源。
- 解析状态、学习状态、标签和分类。
- 题目关联的图片和对话摘要。

### 任务状态

导入和 AI 任务需要区分：

- `pending`
- `processing`
- `completed`
- `failed`
- `needs_review`

失败响应至少包含稳定错误码、用户可读提示和是否允许重试。

### 学习状态

学习状态必须围绕题目保存，至少包含：

- 掌握度。
- 错题次数。
- 连续答对次数。
- 最近练习时间。
- 下次复习时间。
- 错因。
- 薄弱标签。
- 复习建议。

## 五、本阶段不做的事情

为了保证 MVP 可控，暂时不做：

- 恢复 PDF 上传及试卷拆题。
- 将聊天会话提升为顶层业务对象。
- 前端直连 Python 服务。
- 引入更多独立微服务。
- 社区、分享、班级和复杂权限。
- 没有真实用户需求支撑的 AI 花哨功能。
- 只增加视觉页面但不连接真实数据。

## 六、下一轮具体执行任务

下一轮按以下顺序执行：

1. 运行前端 build、Go test、Python test，并记录当前结果。
2. 检查四个服务的本地启动说明和环境变量模板。
3. 准备一组真实或演示题数据。
4. 完成一次端到端导入、校准、作答、解析、追问、归纳、推荐验收。
5. 输出当前最影响可用性的 5 个问题。
6. 优先修复其中的阻塞问题，再补充演示和交付文档。

本阶段的最终验收标准是：

> 用户能在桌面端稳定完成“导入一道错题 → 修正题面 → 作答 → 获得解析 → 继续追问 → 归纳错因 → 获得复习推荐”，并且任何一步失败后都有明确的恢复路径。
# 2026-08-19 P0 实现与验收进展

## 已完成

- 统一导入和 AI 任务状态契约，使用 `pending`、`processing`、`completed`、`failed`、`needs_review`，移除旧的 `queued` 状态。
- OCR 结构质量需要人工校准时，批量导入项和 OCR Job 进入 `needs_review` 终态，不再被前端误判为处理中或超时。
- 批量导入前端轮询把 `needs_review` 视为可结束状态，并能打开对应题目继续人工校准。
- 追问失败消息增加“重新发送”入口；解析失败仍支持“重新解析”。
- 新增任务状态契约单元测试，覆盖 `pending` 和 `needs_review`。

## 验证结果

- 前端：`npm run build` 通过。
- 前端：`npm test -- --watchAll=false` 通过，工作台测试已适配路由上下文和 Markdown ESM 依赖。
- Go：`go test ./...` 通过。
- Python：`python -m unittest discover -s tests` 通过，7 个测试通过。
- AI 服务 mock 运行态：健康检查、mock 解析和图片 OCR 均成功。

## 当前验收阻塞

- 本机 MySQL84 服务和 3306 端口正常，但项目示例凭据 `root:password` 认证失败，Go 服务无法连接数据库，因此暂时不能完成 Go API 级别的端到端验收。
- 下一步需要提供本机 MySQL 的有效开发凭据，或由用户先在本机创建/授权项目数据库用户；不应擅自修改数据库密码。

## 解除阻塞后的验收顺序

1. 启动 Go 服务并确认自动建表成功。
2. 通过 Go API 导入演示图片，轮询 Job 和批量导入状态。
3. 验证题目校准、作答、解析、追问、错因归纳和学习状态落库。
4. 创建推荐练习并提交，核对统计页和归档页数据联动。
# 2026-08-19 Go API 端到端验收完成

## 本轮实际联调结果

- 使用用户本机正在运行的 `8001`、`8080`、`3000` 完成健康检查；确认之前的数据库失败是 Agent 手动覆盖了错误 DSN 导致的。
- 通过 Go API 导入演示图片，单题导入成功，OCR Job 为 `completed`，题干和 4 个选项成功落库。
- 批量导入 2 张图片成功，两个批次项均进入 `completed`。
- 对新题执行人工校准，`correctAnswer`、`parseSource=manual_corrected`、`qualityStatus=ok` 返回正确。
- 使用 mock AI 联调实例跑通人工校准后的重新解析，解析结果返回答案、摘要、知识点、逐步解析和易错点。
- 追问已通过 Go → Python 完成题目上下文绑定，并返回包含答案、知识点和当前追问内容的非占位回复。
- 错因归纳成功落库，练习会话成功创建，客观题自动判分为正确，学习状态和下次复习时间更新。
- 推荐接口返回今日复习、最近错题、薄弱专项、新题巩固、随机混合五组结果。
- 分类筛选和标签筛选均通过 API 返回匹配题目。

## 本轮修复

- Python mock Chat 不再返回“未配置”的占位语，改为根据当前题目、已有解析、历史轮次和用户追问生成有上下文的 mock 回复。
- 新增 mock Chat 单元测试，验证答案、追问轮次和问题内容会进入回复。

## 验证结果

- 前端：`npm run build` 通过。
- 前端：`npm test -- --watchAll=false` 通过，1 个测试通过。
- Go：`go test ./...` 通过。
- Python：`python -m unittest discover -s tests` 通过，8 个测试通过。
- 前端源码未发现 Python AI 服务地址或 `/internal/v1` 直连。
# 2026-09-05 当前进度与 Agent 化路线判断

## 本轮已执行

- 图片上传改为先写入持久化本地对象存储适配器，再创建数据库 Job；单题和批量导入不再在请求线程中等待 OCR。
- 增加数据库 worker、任务租约、自动退避重试、过期 processing 接管、手动重试接口和 OCR 重试入口；默认 3 个 worker，可通过环境变量调整。
- OCR 空结果、AI 解析失败、聊天调用失败和聊天附件保存失败均有明确失败路径；前端支持重试识别、重试解析和真正重新发送聊天消息。
- 当前对象存储实现是可替换抽象，默认是本地持久化实现；S3/MinIO 适配器尚未接入，不在本轮伪装成已完成。
- 未实现登录注册、解析版本记录和 AI 分类标签确认应用；相似题仍保留为后续 Agent 工具能力评估项。

## 验证

- Go 测试通过，Python 测试 8 项通过，前端测试和生产构建通过。
- 新后端备用端口启动成功，完成迁移并读取现有数据库 19 道题；未新增或修改题目数据。

---

## 当前判断

- 项目已经进入 MVP 收口阶段：前端三栏错题工作台、Go 业务端、Python AI 服务和围绕 Question 的导入—OCR—校准—作答—解析—追问—错因—复习推荐链路已形成。
- 当前能力更准确的定位是“题目工作流 + 题目上下文 AI 助手”，还不是可自主规划、调用工具、持久执行并在关键动作前请求确认的 Agent。
- 现阶段主要缺口是：真实 OCR/LLM 质量验收、多用户鉴权、可恢复的持久化任务执行、工具调用与动作确认、长期记忆/检索、主观题批改、Agent 评测与可观测性。

## 架构结论

- 暂不因“Agent”概念引入 LangChain/LangGraph，也不新增独立微服务；当前直接调用模型的实现更适合先把真实链路和业务边界做稳定。
- 后续若出现多步规划、工具路由、人工确认、暂停恢复和长任务编排，再在现有 Python AI 服务内部增加 Agent Runtime；LangGraph 可用于状态图和 checkpoint，LangChain 只在需要统一模型/工具/检索抽象时引入。
- Go 继续作为唯一业务入口、权限与状态落库方；Python 负责 Agent 推理、工具编排建议和结构化 AI 结果，不直接写主业务库。

## 推荐顺序

1. 先完成真实题目端到端验收和失败恢复。
2. 补齐鉴权/用户隔离、持久化任务、幂等重试、附件与对象存储。
3. 定义受控工具契约：读取题目/解析、搜索错题、保存错因、更新学习状态、生成练习草稿；涉及写入的动作必须支持确认。
4. 先用显式状态机实现 Agent MVP，再根据复杂度决定是否引入 LangGraph。

---
# 2026-09-09 下一步功能改进建议

## 推荐 commit message

`feat: 完成 taxonomy 闭环与 P1 单题辅导能力`

建议正文：

- 接通受候选约束的 taxonomy 建议生成与用户确认应用。
- 支持真实图片字节的多模态题目追问及幂等重试。
- 增加相似题候选 proposal 的预览、拒绝、过期校验和确认创建。
- 增加带评分依据和人工确认边界的主观题辅助批改。
- 补充 Go/Python/前端契约测试、24 例 P1 离线评测和接口文档。

## 下一步功能改进顺序

1. 先补真实闭环验收：配置授权的视觉/LLM provider 和测试 MySQL，完成真实图片追问、相似题、主观题批改各至少 2 例；记录耗时、失败率、token 和人工复核结果。
2. 完善身份与数据归属：接入真实鉴权，将当前固定用户替换为登录用户；所有题目、proposal、附件、练习会话和 taxonomy 操作都按用户权限校验。
3. 做好题目质量回路：补充 OCR/解析人工校准、解析失败重试、题目版本历史，以及题面或作答变更后相关分析、proposal、评分建议的自动失效提示。
4. 提升复习价值：让用户可以编辑 AI 生成的错因、薄弱点、标签和复习建议，并把人工确认结果沉淀到学习状态和复习队列；AI 建议仍不能直接覆盖人工结果。
5. 补齐可用性细节：增加图片压缩/缩略图、上传进度、视觉分析超时提示、批量重试、proposal 历史和撤销/恢复反馈；移动端只做必要的降级布局，不改变桌面三栏工作台。
6. 建立受控质量评测：扩充真实授权样例，分别评估 taxonomy 越界、图片信息利用、相似题非机械复制、批改证据覆盖和人工采纳率；模型效果与 mock 契约结果分开统计。

暂不建议下一步引入知识图谱、向量库、社区分享、PDF 主线或更多微服务；先把真实 provider、鉴权、人工校准和复习反馈做成可持续闭环。

---
# 2026-09-10 当前问题分析与下一步方案

本轮只做问题分析和方案设计，不执行代码修改。

## 1. 图片上传后题目空白、解析结果不自动出现

### 判断

这是“异步任务状态与前端刷新策略不完整”叠加“OCR 需要复核时会阻断分析”的问题：

- 前端上传成功后立即加载刚创建的 Question，此时题干、选项和解析本来可能为空。
- 单图导入虽然有轮询，但先把导入状态设为结束，轮询期间没有持续刷新题目详情；只在最终状态时更新一次。
- 当前轮询间隔为 5 秒，分析接口暂时没有结果时被当作普通空结果处理，用户看不到明确的 processing 状态。
- Go worker 在 OCR 为 `needs_review` 时直接不入队 AI 分析。如果 OCR 仍有可用题干，这会让用户误以为系统卡住。

### 推荐方案

第一阶段先使用可靠轮询，不立即引入 WebSocket：上传接口返回 `jobId/questionId` 后，前端进入统一的导入任务状态机，每 1～2 秒读取任务状态和题目摘要，在 `ocr_processing`、`analysis_pending`、`analysis_processing` 时实时更新中栏；解析完成后再请求完整题目和分析。轮询必须可取消、按任务 ID 去重，并在组件切换题目时停止旧任务。

后端保留并统一以下状态：`queued`、`processing`、`completed`、`needs_review`、`failed`，分别记录 `ocrStatus`、`analysisStatus` 和 `processingStage`。分析接口暂时无数据不能解释成“无解析”，应显示“解析中”。

OCR `needs_review` 建议分两种：题干不可用时停止并要求人工校准；题干可用但置信度不足时继续分析，同时把最终状态标为 `needs_review`，让用户先看到 AI 结果和质量警告。后续再把轮询替换为 SSE，减少等待延迟。

验收标准：上传后 1～2 秒内显示处理阶段；OCR 完成后自动显示题干；AI 分析完成后无需刷新即可显示解析；失败和待复核状态有明确操作；切换题目或重复上传不会出现旧任务覆盖新题目。

## 2. 用户隔离与个性化 Agent

### 推荐的低门槛做法

不建议一开始强制注册。采用“匿名用户 + 可选升级账号”：首次访问时由 Go 创建匿名 User，并通过签名的 HttpOnly Cookie/Session Token 识别；题目、作答、错因和 Agent 记忆都归属于该匿名用户。用户可以直接使用，不需要填写注册信息；以后需要跨设备同步时，再通过邮箱或 OAuth 将匿名用户升级为正式账号并迁移数据。

需要明确告知用户：未注册数据只绑定当前浏览器/设备，清除 Cookie 或更换设备后无法恢复。服务端不能接受前端传来的任意 `userId`，必须从会话解析用户身份。

### 个性化数据模型

不要把整段历史对话直接塞进 Prompt，而是由 Go 维护有限的学习证据：

- `User` / `UserSession`：匿名或正式身份。
- `Question.userId`、`PracticeSession.userId`、`AIProposal.userId`、附件归属：完成数据隔离。
- `LearningEvidence`：题目、知识点、错因、作答结果、来源和时间。
- `UserSkillProfile`：例如 `osi.seven_layers`、`data_structure.linked_list`，保存掌握度、错误次数、最近证据和复习时间。
- `UserPreference`：解释详细程度、偏好的提示方式、目标难度等。

Agent 每次只接收三层上下文：当前题目、用户偏好、与当前题相关的 Top-N 薄弱知识点及最近证据。比如用户 A 的 OSI 薄弱点只影响 A 的提示和复习推荐，不会进入用户 B 的上下文。Python 只处理 Go 传入的脱敏上下文，学习证据和掌握度仍由 Go 更新。

推荐分三步落地：先做匿名会话隔离，再做知识点/错因证据沉淀，最后做基于证据的 Agent 提示和练习推荐。这样可以避免为了个性化一次性引入复杂注册体系或向量数据库。

## 3. 当前 Agent 是否使用 LangGraph

当前没有使用 LangGraph。实现是 Python 中的显式 `AgentDispatcher`：业务按钮决定 action，代码完成前置校验、调用模型、JSON/schema 校验、领域校验、重试和统一错误返回。这个方案适合当前六个边界清晰、单次请求为主的动作。

LangGraph 是 LangChain 体系中的低层 Agent 编排框架和运行时，核心是把工作流表示为“状态 + 节点 + 边”：可以做条件分支、循环、并行、持久化、流式输出、失败恢复和人工中断；它本身不是模型，也不会自动产生个性化能力。官方定位也强调它主要解决长流程、有状态 Agent 的编排，而不是替开发者决定 Prompt 或业务架构。[LangGraph 官方概览](https://docs.langchain.com/oss/python/langgraph/overview)

放到 ErroNotebook 中，未来可以把一次复杂辅导编成：

`读取题目/用户画像 → 质量检查 → 选择辅导策略 → 调用模型 → 结构校验 → 证据校验 → 需要补充信息或人工确认 → 返回 Go`

但不建议现在立即引入。LangGraph 不能替代 Go 的用户隔离、数据库事务和最终状态控制；当前图片追问、相似题 proposal、主观题评分建议已经可以用显式 dispatcher 可靠完成。只有当项目需要多步推理、流式节点状态、长任务恢复、人工中断后继续或多个专用 Agent 协作时，再考虑在 Python 内部引入 LangGraph，并保持“前端只访问 Go、Python 不写业务库”的边界。

---
# 2026-09-10 匿名用户隔离与图片导入异步状态修复 Goal 指令

以下内容可直接作为下一轮 `/goal` 指令。本轮只生成指令，不执行实现。

```text
请以“实现 ErroNotebook 匿名用户隔离基础与图片导入异步状态修复”为本次 goal。请实际完成代码、测试、验收和必要文档，不只输出方案。先完整阅读 AGENTS.md、project.md、webdesign.md、talk.md 最新记录、backend/API.md、ai-service/README.md 及当前实现；先检查工作区并保留已有修改，不执行 reset、clean 或覆盖无关变更。

一、范围和边界
1. 本轮只完成两件事：
   - 修复图片上传后题目空白、OCR/AI 解析期间不自动更新的问题。
   - 建立不强制注册的匿名用户身份与后端数据隔离基础。
2. 不实现邮箱注册、密码、OAuth、跨设备账号迁移、完整用户画像、向量库、知识图谱或 LangGraph；为后续升级账号和个性化 Agent 预留清晰接口即可。
3. 保持前端只访问 Go；Go 负责身份、权限、任务状态和数据库；Python 只负责 OCR、LLM、多模态结果，不读取或写入业务数据库。

二、先确认基线
1. 运行并记录 Python 测试、Go 测试、前端测试/构建和 P0/P1 离线评测。
2. 检查当前图片导入、Question/Job 状态、worker、题目详情接口和前端 `QuestionWorkbenchPage` 的实际竞态，确认原有修改不被覆盖。
3. 所有新状态和错误必须兼容当前 API；不要用前端猜测状态或把“暂无结果”当成“解析完成”。

三、修复图片上传和异步状态
1. 上传接口继续返回 `jobId`、`questionId`。Go 为任务维护可观察的 `queued`、`processing`、`completed`、`needs_review`、`failed`，并分别暴露 OCR 阶段、AI 分析阶段和错误信息。
2. 优先使用可取消、按 job/question 去重的 1～2 秒轮询，不在本轮引入 WebSocket；可以新增清晰的任务进度接口，也可以扩展现有 Job 查询接口。
3. 单图和批量导入都必须显示占位题目和明确阶段：上传中、OCR 识别中、解析排队、AI 分析中、待人工复核、失败可重试。任务未完成前不得把 loading 状态提前置为结束。
4. OCR 结束后立即刷新题干、题型、选项和质量状态；AI 分析结束后立即刷新完整解析、taxonomy 建议和对话上下文，无需手动刷新页面。
5. 分析接口在后台尚未生成结果时返回/处理为 processing，不显示空白解析。前端必须防止旧题目的轮询结果覆盖当前选中题目，并在切换题目、组件卸载和重复上传时取消旧轮询。
6. 明确 `needs_review` 策略：题干不可用时暂停并要求校准；题干基本可用但置信度不足时可以继续 AI 分析，最终保留 needs_review 和 warning。失败必须显示错误原因和重试入口。
7. 对任务轮询增加退避、最大等待时长、网络失败重试和终止提示；不得无限轮询。保留现有数据库 worker 的重试和租约语义。

四、匿名用户身份和隔离
1. 增加最小 `User` 与 `UserSession` 模型/迁移。首次访问由 Go 创建匿名用户，使用签名且 HttpOnly 的 Cookie/Session Token 识别；不能信任前端传入的 `userId`，不能把浏览器 localStorage 中的普通 ID 当作权限凭证。
2. 匿名用户不需要注册即可使用。Cookie 丢失、清除浏览器数据或更换设备导致数据不可恢复时，要在产品文案中明确说明。预留未来将匿名用户升级为正式账号的接口，但本轮不实现注册和 OAuth。
3. 增加 Go 鉴权中间件和 request context 中的当前用户。替换当前固定用户 `1` 的写入和查询路径，至少覆盖 Question、QuestionAsset、ChatMessage、Analysis、Job、BatchImport、PracticeSession、PracticeSessionQuestion、QuestionLearningState、AIProposal 及相关附件对象。
4. 所有详情、列表、更新、删除、聊天、proposal 确认/拒绝、练习和推荐接口都必须按当前匿名用户做归属校验；跨用户访问统一返回 404 或明确的无权限错误，不泄露资源是否存在。
5. 处理当前全局 Category/Tag 模型：本轮必须明确选择并实现一种安全策略：
   - 作为全局系统词表时，对普通匿名用户只读，写操作改为受控管理入口；或
   - 增加 owner/scope 字段和迁移，使用户自定义 taxonomy 与系统词表隔离。
   不能让一个匿名用户新增、删除或修改全局分类标签而影响其他用户。
6. 对象存储 key 必须带用户/题目作用域，读取附件前验证归属；禁止通过客户端传入路径、URL 或 userId 越权读取。
7. 不在本轮实现完整 UserSkillProfile，但在学习证据和 Agent 请求中保留从 Go 注入当前用户上下文的扩展点；不得把用户 A 的题目、对话或错因发送给用户 B。

五、测试和验收
1. 增加匿名会话测试：首次访问创建稳定匿名用户、同一 Cookie 复用用户、不同 Cookie 用户隔离、Cookie 缺失重新创建、伪造 userId 无效。
2. 增加资源归属测试：题目、聊天、附件、分析、Job、proposal、练习会话和推荐不能跨用户读取或修改；重复请求保持幂等。
3. 增加异步前端/接口测试：新题目占位、OCR processing、analysis processing、完成后自动刷新、needs_review、失败重试、切换题目取消旧轮询、网络暂时失败和超时。
4. 保留并通过现有 P0/P1 测试、Go→Python multipart 图片契约测试和前端生产构建；不要以 mock 结果替代异步状态或用户隔离测试。
5. 验收必须证明：上传图片后无需刷新即可看到题干和 AI 解析；不同匿名用户看不到彼此题目和附件；Python 仍不会接触业务用户表；没有注册流程也能完成导入、作答、追问和练习。

六、文档交付
1. 同步 backend/API.md、ai-service/README.md、project.md、webdesign.md，写清匿名 Cookie、资源归属、任务状态、轮询接口、错误码和匿名数据丢失提示。
2. 从 talk.md 第一行置顶记录实现内容、测试结果、匿名隔离边界、未实现的账号升级和后续个性化画像计划，旧记录整体保留。
3. 最终报告必须列出关键文件、数据库迁移、接口示例、测试命令和结果、已知限制；只有代码和测试均完成后才能宣布 goal 完成。
```

---
# 2026-09-10 匿名用户隔离与图片导入异步状态修复实现记录

## 本轮已完成

- Go 新增最小 `User`/`UserSession` 模型与 AutoMigrate；首次 API 访问创建匿名用户，使用签名 HttpOnly、SameSite=Lax 的 `erro_session` Cookie，前端请求改为 `credentials: include`。
- Question 为归属根，题目、附件元数据、分析、Job、聊天、批量导入、练习会话/题目、学习状态和 AI proposal 均按匿名用户隔离；跨用户题目/会话/Job 返回 404。对象存储路径改为 `users/{userId}/questions/{questionId}/...`。
- Category/Tag 本轮选择全局系统词表只读策略：匿名用户可读取和绑定已有值，不可新增、修改或删除共享分类标签。
- 图片任务统一使用 `queued`、`processing`、`completed`、`needs_review`、`failed`；worker 区分 OCR/AI 阶段，低置信度但题干可用时继续 AI，题干不可用时停在校准；旧 `pending` Job 仍可接管。
- 前端单图/重新解析使用可取消、去重、退避、5 分钟上限轮询；上传立即显示占位题目，OCR 后增量刷新题干/选项，AI 后刷新解析/对话，解析未生成时显示“解析中”。切题、重复上传和卸载组件不会让旧结果覆盖当前题目。

## 测试与验收

- 基线：Python `python -m pytest -q` 19/19；Go `$env:GOCACHE=(Join-Path (Get-Location) '.gocache'); go test ./...` 通过；前端 `npm test -- --watchAll=false --runInBand` 1/1 通过。
- 新增 Go auth 单测覆盖首次建匿名会话、同 Cookie 复用、无 Cookie 隔离、篡改 Cookie 失效及匿名数据丢失提示；Go 全量测试通过。
- Question/Job/Batch/练习/学习状态和 proposal 已提供 owner-scoped 查询/确认路径；HTTP question/session middleware 在所有相关操作前执行归属检查。
- 未配置 MySQL/真实 provider 的环境仍不能宣称真实模型质量或数据库端到端已验证；下一步应用两份真实 Cookie 完成上传、作答、追问和练习演示，并核对附件与 Job 的跨用户 404。

## 明确限制和后续

- 本轮不实现邮箱注册、密码、OAuth、正式账号升级、UserSkillProfile、向量库、知识图谱或 LangGraph。
- 本轮不引入 WebSocket；轮询最长 5 分钟，网络连续失败会提示后台任务仍在处理。
- 未来账号升级只迁移匿名 UserSession，不改变 Question 及其关联实体的 owner 模型。
# 2026-09-13 单用户单实例化：移除注册、登录与用户 Session

## 本轮决策

- 项目正式按单实例本地系统实现，不再保留注册、登录、账号、OAuth、匿名 Cookie、浏览器 session 或多用户数据隔离。
- 保留“练习会话”作为业务对象；它不是用户身份 session。
- Go 不再创建/校验 `User`、`UserSession`，不再提供 `/api/v1/session`，前端请求不再携带 `credentials`。
- 业务模型和仓储层移除运行时 `user_id` 归属及 `ForUser` 查询；题目、解析、聊天、练习、学习状态、分类和标签直接属于当前部署实例。
- 图片对象 key 统一为 `questions/{questionId}/...`。

## 已完成

- 删除 Go auth/session 实现与用户模型，移除 session 配置项和 Cookie CORS 配置。
- 移除前端匿名提示、`/session` 请求、用户头像语义和 `credentials: include`。
- 将仓储、服务、worker、taxonomy、proposal 和练习链路改为无用户过滤的单实例访问。
- 同步 `README.md`、`backend/README.md`、`backend/API.md`、`project.md`、`webdesign.md`、`ai-service/README.md` 与 `.env.example`。
- Go 与前端测试已通过；后续需要在已有 MySQL 数据库上验证旧表/旧索引迁移，并确认旧上传文件是否需要一次性从 `users/{userId}/...` 移到新路径。

## 当前边界

如果未来需要多人共享或账号恢复，应另行设计租户/账号体系；本阶段不通过隐藏 Cookie 或固定 owner 继续保留身份概念。
