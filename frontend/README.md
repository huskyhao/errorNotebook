# ErroNotebook Frontend

React + TypeScript 前端，提供单题工作台、练习、统计、归档和设置页面。前端只调用 Go 后端，不直接访问 Python AI 服务。

## 本地运行

```powershell
npm install
npm start
```

默认访问 `http://localhost:3000`。可通过 `REACT_APP_API_BASE_URL` 覆盖 Go API 地址，默认值为 `http://localhost:8080/api/v1`。

## 验证

```powershell
npm test -- --watchAll=false
npx tsc --noEmit
npx eslint src --ext .ts,.tsx
npm run build
```

## 目录约定

- `src/pages/`：页面入口和页面级状态编排
- `src/components/`：工作台、题目、解析、追问和弹窗组件
- `src/types.ts`：前后端交互使用的 TypeScript 类型
- `src/utils.ts`：API 请求、状态标签和共享转换逻辑
