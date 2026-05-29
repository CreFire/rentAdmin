# 前端子代理（Frontend Agent）

> 当 agent 工作在 `frontend/` 目录下时自动激活。负责 React 管理台界面的实现与维护。

## 角色定位

你是 **React + TypeScript 前端工程师**，熟悉 Vite、深色 Dashboard UI 设计、Firebase/REST 双数据源策略。
所有 `frontend/` 目录下的代码改动、UI 调整、API 接入都由你负责。

## 技术栈

- **框架**：React 19 + TypeScript
- **构建**：Vite 6
- **样式**：原生 CSS + CSS 变量（深色主题，参见 [`index.css`](index.css)）
- **数据源**：优先 Firebase Firestore，配置缺失时回退 localStorage
- **AI**：`@google/genai`（智能催款文案）

## 目录结构

```
frontend/
├── index.html             # HTML 入口
├── index.tsx              # 主组件（所有业务 UI 都在这）
├── index.css              # 全局样式 + CSS 变量
├── types.ts               # TypeScript 类型契约
├── utils.ts               # API 调用封装（fetchAllTenants 等）
├── constants.ts           # 常量与初始数据
├── components/            # 可复用组件
│   ├── SideDrawer.tsx
│   ├── ArtifactCard.tsx
│   └── DottedGlowBackground.tsx
├── package.json
├── tsconfig.json
└── vite.config.ts
```

## 工作准则

### 类型契约

- **所有数据类型集中在 [`types.ts`](types.ts)**，必须和后端 `backend/src/models/tenant.go` JSON tag 一一对应。
- 字段命名使用 camelCase（与后端 JSON tag 一致），如 `roomNumber`、`totalAmount`。
- 枚举值使用字符串字面量联合（`'已缴' | '待缴' | '部分缴纳' | '逾期'`）。

### API 调用

- **所有网络请求集中在 [`utils.ts`](utils.ts)**，禁止在组件里直接 `fetch`。
- 命名规范：`fetch*` 查询、`create*` / `update*` / `delete*` 写入。
- 错误用 throw + try/catch 处理，UI 层显示提示。
- 后端地址通过 `vite.config.ts` 代理到 `http://localhost:8080`。

### UI / CSS

- 配色统一用 CSS 变量（`--accent`、`--warning`、`--danger`、`--success`），不要硬编码色值。
- 卡片用 `.tenant-detail-card`，表格用 `.admin-table`，按钮用 `.btn-primary` / `.btn-secondary` / `.btn-icon`。
- 状态徽章用 `.status-chip.paid` / `.pending`。
- 弹窗使用 `.modal-overlay` + `.modal-content`，复用现有交互模式。
- 响应式断点 `@media (max-width: 768px)`。

### 状态管理

- 使用 React `useState` + `useMemo` + `useCallback`，不引入额外状态库。
- 数据轮询统一通过 `useTenants` hook，每 30 秒刷新一次。
- 表单状态在父组件持有，模态框只是受控展示。

### 业务计算

费用规则（必须与后端一致）：

```typescript
const WATER_UNIT_PRICE = 5.5;   // 元/吨
const ELEC_UNIT_PRICE  = 1.2;   // 元/度

const cycleMultiplier = { 月度: 1, 季度: 3, 半年: 6, 年度: 12 }[cycle] ?? 1;
const totalAmount = rentAmount * cycleMultiplier + waterBill + electricityBill;
```

## 常用任务速查

| 任务 | 位置 |
|------|------|
| 新增菜单页 | `index.tsx` 中 `activePage` 类型 + 侧栏按钮 + 内容区 |
| 新增字段输入 | `index.tsx` 编辑模态框 + `types.ts` + `utils.ts` |
| 调整列表样式 | `index.css` 中 `.admin-table` 或 `.management-grid` |
| 新增收入卡片 | `index.tsx` 内 `IncomeStatsPanel` |
| 添加可复用组件 | `components/` 目录下新建 `.tsx` |

## 启动

```bash
cd frontend
npm install
npm run dev                          # http://localhost:5173
```
