# 租房前端数据与外部 API 说明

本文梳理当前前端项目（`rentadmin-minimal`）中涉及的所有数据接口、调用方式及字段约定，便于后续对接或排查。

## 数据源切换策略
- **优先 Firestore**：`index.tsx` 中的 `firebaseConfig` 只要不以 `"YOUR_"` 开头即视为已配置；初始化后使用 `getFirestore`。
- **本地模拟**：配置缺失或初始化异常时，自动退化到 `localStorage`，键名 `tenants_mock_db`，并以 `INITIAL_RECORDS` 作为种子数据。
- **同步机制**：本地模式写入后会触发浏览器事件 `local-db-update`，以便组件刷新。

## Firestore 访问点
集合名称：`tenants`

| 操作 | 方法 | 触发位置 | 说明 |
| ---- | ---- | -------- | ---- |
| 订阅列表 | `onSnapshot(query(collection(db,"tenants"), orderBy("roomNumber","asc")))` | `useTenants` hook | 实时拉取房间号升序数据。 |
| 新增记录 | `addDoc(collection(db,"tenants"), data)` | `addRecord` / 计量录入无当月记录时 | 记录新增的账单/入住信息。 |
| 更新记录 | `updateDoc(doc(db,"tenants", id), data)` | `updateRecord`、缴费、计量更新、表单编辑 | 按 `id` 局部更新。 |

> 本地模式下，以上三类操作分别映射为数组读写与替换，保持字段一致。

## 数据模型（`types.ts`）
`TenantRecord` 字段：
- `id: string`
- `roomNumber: string`  — 房号
- `name: string`
- `phone: string`
- `idCard?: string`
- `checkInDate?: string` (YYYY-MM-DD)
- `deposit?: number`
- `rentAmount: number`
- `waterReading: number`（当前水表）
- `electricityReading: number`（当前电表）
- `waterBill: number`
- `electricityBill: number`
- `totalAmount: number` — 本期合计（租金 + 水电）
- `amountPaid: number` — 已收
- `rentCycle: '月付'｜'季度'｜'半年'｜'年付'`*
- `utilityCycle: 同上`
- `status: '已结清'｜'待收款'｜'逾期'｜'部分已付'`
- `date: string` (YYYY-MM) — 账期
- `recordedAt?: string` (YYYY-MM-DD) — 抄表实际日期

\* 文件中为中文编码字符串，含义如上。

## 费用计算约定
- 单价常量：水 `5.5` 元/吨，电 `1.2` 元/度（`index.tsx` 顶部 `WATER_UNIT_PRICE` / `ELEC_UNIT_PRICE`）。
- 当月水电用量 = 当月读数 − 上一条早于当月的读数（同房号、`date` < 当月）。
- 本期合计 `totalAmount = rentAmount + waterBill + electricityBill`。
- 缴费后状态：`已结清`（全额）、`部分已付`（>0 且 < 合计）、`待收款`（0）。

## 前端业务操作与数据写入
- **新建/编辑租客**：`openEdit` 弹窗提交 -> `handleFormSubmit` -> 新增或更新一条当月账单，计算当期水电费用。
- **抄表录入**：`handleMeterSubmit` 使用表单读数计算水电费用；如当月已有记录则更新，否则新增一条，`amountPaid` 置 0。
- **收款登记**：`handlePaySubmit` 将输入金额累加到 `amountPaid`，并更新 `status`。
- **历史查询**：`history` 页面按房号筛选并按 `date` 降序展示。

## AI 智能催缴（Google GenAI）
- 入口：`SmartReminderModal`（催缴任务卡片 “智能提醒” 按钮）。
- SDK：`GoogleGenAI`（`@google/genai`）。
- 调用：`models.generateContent({ model: 'gemini-3-flash-preview', contents: prompt })`。
- Prompt 含：租客姓名、房号、账期、未付金额。
- 密钥：代码使用 `process.env.API_KEY`，示例 `.env.local` 提供 `GEMINI_API_KEY=PLACEHOLDER_API_KEY`（需对齐变量名）。

## 配置清单
- Firestore：在 `index.tsx` 中填充 `firebaseConfig` 的 `apiKey` / `projectId` / `authDomain` 等。
- Gemini：为前端打包环境注入 `API_KEY`（或调整代码使用 `GEMINI_API_KEY`）。

## 本地模拟数据
- 位置：`constants.ts` 的 `INITIAL_RECORDS`。
- 作用：当未配置 Firebase 时作为初始种子写入 `localStorage`，后续读写均在本地完成。

## 相关文件速览
- `index.tsx`：数据源初始化、主要业务逻辑与 UI。
- `types.ts`：数据模型定义。
- `constants.ts`：种子数据。
- `utils.ts`：`generateId`（未在当前流程使用）。 

以上即现有前端的 API 与数据读写约定，可直接作为对接或联调参考。
