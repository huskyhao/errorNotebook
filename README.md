# ErroNotebook

ErroNotebook 是一个围绕单题展开的 AI 错题工作台。项目重点不是泛化题库，也不是普通聊天机器人，而是把一道题从导入、OCR 结构化、作答、AI 解析、继续追问到保存沉淀的学习闭环跑通。

当前仓库已经从纯规划阶段进入 MVP 收口阶段，保留 Practice、Stats、Settings、Archive、推荐练习、学习状态、错因归纳等现有规划和入口。

## 当前功能

- 题目导入：支持图片上传和手动文本创建，不把 PDF 导入作为当前 MVP 主线。
- OCR 结构化：前端请求 Go 后端，Go 调用 Python AI 服务完成图片识别、题面结构化和质量提示。
- 题目管理：支持题目列表、详情、编辑、删除、收藏、分类和标签。
- 作答与解析：支持提交答案、触发 AI 解析、查看最新解析。
- 单题追问：围绕当前题继续提问，聊天记录挂在题目下。
- 学习状态：记录掌握度、错题次数、连续答对、下次复习时间、错因和薄弱点；这些数据服务于统计、归档和推荐，不在首页详情重复展示。
- 分类与标签：每题一个顶层学科分类，可有多个知识点标签；AI 只提供待确认的分类/标签建议，最终由 Go 校验并生效。
- 推荐练习：提供今日复习、最近答错、薄弱专项、新题巩固、随机混合等面向当前行动的练习入口。
- 页面入口：包含单题工作台、做题模式、学习统计、错题归档和设置页，三者职责分离。

## 架构

```text
React frontend :3000
        |
        v
Go + Gin backend :8080
        |
        +--> MySQL
        |
        v
Python FastAPI ai-service :8001
```

服务边界：

- `frontend/`：React + TypeScript 前端，只调用 Go 后端 API。
- `backend/`：Go + Gin + GORM 业务后端，是前端唯一业务 API 入口，负责数据持久化、状态流转和 AI 服务编排。
- `ai-service/`：Python + FastAPI 内部 AI 服务，负责 OCR、题目结构化、解析生成和追问生成，不直接承担用户体系或主业务数据库写入。

## 目录结构

```text
ErroNotebook/
├── ai-service/       # Python FastAPI AI 服务
├── backend/          # Go + Gin 业务后端
├── frontend/         # React + TypeScript 前端
├── webview/          # 页面视觉参考图
├── project.md        # 产品、架构、接口和数据模型详细规划
├── webdesign.md      # 前端页面、布局、交互和视觉规划
├── talk.md           # 协作记录、阶段结论和推进建议
├── README.md         # 项目入口说明
└── LICENSE           # MIT License
```

## 环境要求

- Node.js 18+
- Go 1.26+
- Python 3.10+
- MySQL 8.0+

默认本地端口：

| 服务 | 端口 |
| --- | --- |
| frontend | `3000` |
| backend | `8080` |
| ai-service | `8001` |
| MySQL | `3306` |

## 启动前准备

> 安全边界：仓库只提交 `.env.example` 模板，不提交任何 `.env`、API key、数据库密码、上传文件或本地构建产物。复制模板后，只在本机填写真实配置；提交前请再次运行 `git status` 和敏感信息扫描。

创建 MySQL 数据库：

```sql
CREATE DATABASE erro_notebook CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

配置 AI 服务：

```powershell
cd ai-service
Copy-Item .env.example .env
```

默认配置可使用 mock OCR / mock LLM 跑通链路。如需接入真实模型，配置：

```text
OPENAI_BASE_URL=
OPENAI_API_KEY=
OPENAI_MODEL=
VISION_BASE_URL=
VISION_API_KEY=
VISION_MODEL=
```

只想先验证 AI 服务时，保持 `LLM_BACKEND=mock`；`OCR_BACKEND=auto` 在未安装 PaddleOCR 或初始化失败时会回落到 mock。真实模型只应通过本地 `.env` 配置，不能把值写入源码或文档。

配置 Go 后端：

```powershell
cd backend
Copy-Item .env.example .env
```

根据本地 MySQL 修改：

```text
PORT=8080
MYSQL_DSN=root:password@tcp(127.0.0.1:3306)/erro_notebook?charset=utf8mb4&parseTime=True&loc=Local
AI_SERVICE_BASE_URL=http://127.0.0.1:8001
AI_SERVICE_TIMEOUT_SECONDS=15
```

## 本地启动

建议按 MySQL、AI 服务、Go 后端、前端的顺序启动。

启动 AI 服务：

```powershell
cd ai-service
python -m venv .venv
.\.venv\Scripts\Activate.ps1
pip install -r requirements.txt
uvicorn app.main:app --reload --host 0.0.0.0 --port 8001
```

启动 Go 后端：

```powershell
cd backend
go mod download
go run ./cmd/server/
```

启动前端：

```powershell
cd frontend
npm install
npm start
```

访问前端：

- `/`：错题工作台
- `/practice`：练习页
- `/practice/:sessionId`：指定练习会话
- `/stats`：学习统计
- `/archive`：错题归档
- `/settings`：设置页

## 测试与构建

Go 后端测试，推荐使用仓库内缓存：

```powershell
cd backend
$env:GOCACHE=(Resolve-Path ..\.gocache).Path
go test ./...
```

Python 测试：

```powershell
cd ai-service
python -m unittest discover -s tests
```

前端生产构建：

```powershell
cd frontend
npm run build
```

前端开发和测试命令也可以在 `frontend/` 中运行：

```powershell
npm test
```

`frontend/build/`、`node_modules/`、Python 虚拟环境、Go 缓存和测试缓存均为本地生成物，不属于公开仓库边界。

## 当前限制

- 当前没有完整鉴权体系，后端仍以固定业务用户或本地配置为主。
- PDF 导入、试卷拆题、批次校对和整卷作答不属于当前 MVP 主线。
- 部分 AI 能力可通过 mock 跑通，真实 OCR/LLM 效果取决于本地环境变量和模型服务配置。
- 客观题支持基础自动判分，主观题、简答题、论述题和计算题需要人工批改或后续 AI 扩展。
- 前端必须继续通过 Go 后端访问业务能力，不直接调用 Python AI 服务。
- 图片上传已与 OCR/解析解耦：图片先写入持久化对象存储，再由数据库任务 worker 后台排队处理；默认使用本地对象存储适配器。

## 文档入口

- [project.md](./project.md)：产品定位、功能范围、系统架构、接口契约、数据模型和代码基线。
- [webdesign.md](./webdesign.md)：前端页面规划、布局职责、交互状态和视觉方向。
- [backend/API.md](./backend/API.md)：Go 后端当前 API 说明。
- [talk.md](./talk.md)：协作记录、阶段结论和下一步建议。

## License

本项目使用 [MIT License](./LICENSE)。
