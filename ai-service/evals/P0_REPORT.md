# P0 离线评测报告

日期：2026-09-07

## 范围

`p0_cases.json` 共 20 个可追溯样例，覆盖操作系统、计算机网络、数据结构、计算机组成原理四门 408 学科，八类题型，以及缺少作答、残缺选项、答案矛盾、指令注入、非法 role、长历史、taxonomy 无匹配和 provider 故障。

## 可复现结果

| 项目 | 结果 | 说明 |
| --- | --- | --- |
| Python 单元/契约/故障测试 | 15 passed | `python -m pytest -q` |
| 四类动作离线 smoke | 4/4 completed | `python evals/run_p0_eval.py`，来源均为 `mock` |
| 评测案例加载 | 20/20 | 脚本输出 `caseCount=20` |
| Go→Python 动作请求响应契约 | passed | `backend/internal/integrations/ai/client_test.go` 使用 httptest |
| Go 全量测试 | passed | `GOCACHE` 指向工作区可写目录后执行 `go test ./...` |
| 前端生产构建 | passed | `npm run build` |

## 契约指标

固定样例中，非法 role、mock 冒充真实、失败伪装 completed、非法 taxonomy 越界、过期指纹覆盖和重复 taxonomy 创建均为 0 次。动作输出会记录来源、attempts、版本指纹和可用 usage；当前 mock usage 为 `null/0`，不代表真实 token 用量。

## 未验证项

当前工作区未配置获授权的真实 provider、MySQL 联调数据和浏览器端真实点击录屏，因此没有宣称真实模型内容质量、真实耗时/token、数据库事务端到端或真实中栏网络链路已经验收。执行真实验收时必须使用有限预算且不打印密钥；P1 图片追问、相似题生成、主观题辅助批改不属于本报告范围。
