# RentAdmin 房产管理系统

## 项目概述

RentAdmin 是一个用于管理房产租赁业务的全栈 Web 应用程序，提供租户管理、账单管理、水电抄表、收入统计和智能催款提醒等功能。

## 技术栈

### 前端
- **框架**: React 19 + TypeScript
- **构建工具**: Vite 6
- **样式**: CSS (自定义样式 + CSS 变量)
- **可选数据源**: Firebase Firestore (支持本地 localStorage 回退)
- **AI 集成**: Google Gemini API (智能催款文案生成)

### 后端
- **语言**: Go 1.25
- **框架**: Gin (HTTP 路由框架)
- **数据库**: SQLite (使用 WAL 模式)
- **Excel 处理**: excelize/v2

### 部署
- Docker & Docker Compose
- 前端开发服务器: http://localhost:5173
- 后端 API 服务器: http://localhost:8080

## 项目结构

```
rentadmin/
├── backend/                    # Go 后端服务
│   ├── main.go                # 程序入口点
│   ├── log_config.go           # 日志配置
│   ├── src/
│   │   ├── models/
│   │   │   └── tenant.go      # TenantRecord 数据模型
│   │   ├── database/
│   │   │   ├── db.go          # 数据库连接管理
│   │   │   └── migrations.go  # 数据库迁移脚本
│   │   ├── handlers/
│   │   │   ├── tenant_handler.go  # 租户相关 HTTP 处理器
│   │   │   └── excel_handler.go   # Excel 导入处理器
│   │   ├── routes/
│   │   │   └── router.go      # API 路由配置
│   │   ├── services/
│   │   │   ├── excel_import.go    # Excel 导入服务
│   │   │   └── excel_import_test.go
│   │   ├── utils/
│   │   │   ├── income_calculator.go  # 收入计算工具
│   │   │   └── logging.go            # 日志工具
│   │   ├── api/
│   │   │   └── api.go           # API 包 (空文件)
│   │   └── router.go           # 路由 (空文件,已迁移)
│   ├── tools/
│   │   └── generate_excel_template.go  # Excel 模板生成工具
│   ├── conf/
│   │   └── global.yaml         # 全局配置
│   ├── bin/                    # 编译输出和数据库文件
│   │   ├── rentadmin.db        # SQLite 数据库
│   │   └── rentadmin.exe       # 编译后的可执行文件
│   └── go.mod / go.sum         # Go 模块依赖
│
├── frontend/                   # React 前端应用
│   ├── index.tsx               # 主 React 组件 (包含所有 UI 逻辑)
│   ├── index.html              # HTML 入口文件
│   ├── index.css               # 全局样式
│   ├── types.ts                # TypeScript 类型定义
│   ├── utils.ts                # API 调用工具函数
│   ├── constants.ts            # 常量定义和初始数据
│   ├── package.json            # 前端依赖
│   ├── tsconfig.json           # TypeScript 配置
│   ├── vite.config.ts         # Vite 配置
│   └── components/             # UI 组件 (预留)
│
├── docker-compose.yml          # Docker 编排配置
├── Agents.md                   # 本文件
└── package-lock.json           # 根目录锁文件 (用于 Python 启动脚本)
```

## 数据模型

### TenantRecord (租户记录)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 唯一标识符 |
| roomNumber | string | 房间号码 |
| name | string | 租户姓名 |
| phone | string | 电话号码 |
| idCard | string | 身份证号 (可选) |
| checkInDate | string | 入住日期 YYYY-MM-DD (可选) |
| deposit | number | 押金 (可选) |
| rentAmount | number | 租金金额 |
| waterReading | number | 水表读数 |
| electricityReading | number | 电表读数 |
| waterBill | number | 水费 |
| electricityBill | number | 电费 |
| totalAmount | number | 本期合计 (租金 + 水电) |
| amountPaid | number | 已付金额 |
| rentCycle | PaymentCycle | 租金周期 |
| utilityCycle | PaymentCycle | 水电周期 |
| status | PaymentStatus | 缴费状态 |
| date | string | 账期 (格式 YYYY-MM) |
| recordedAt | string | 抄表日期 (可选) |
| monthlyIncome | number | 月度收入 |
| annualIncome | number | 年度收入 |
| waterElecIncome | number | 水电总收入 |
| monthlyWaterElecIncome | number | 月度水电收入 |
| annualWaterElecIncome | number | 年度水电收入 |
| createdAt | time | 创建时间 |
| updatedAt | time | 更新时间 |

### PaymentCycle 枚举值
- `月度` / `月付` / `月`
- `季度`
- `半年` / `半年度`
- `年度` / `年度缴纳` / `年`

### PaymentStatus 枚举值
- `已缴` - 全额已付
- `待缴` - 未付款
- `部分缴纳` - 部分付款
- `逾期` - 逾期未付

## 费用计算规则

### 单价常量
- 水费: 5.5 元/吨 (`WATER_UNIT_PRICE`)
- 电费: 1.2 元/度 (`ELEC_UNIT_PRICE`)

### 计算公式
```
租金周期倍数:
  - 月度: 1
  - 季度: 3
  - 半年: 6
  - 年度: 12

本期租金 = rentAmount × 租金周期倍数
水费 = (当前水表读数 - 上期水表读数) × 5.5
电费 = (当前电表读数 - 上期电表读数) × 1.2
本期合计 = 本期租金 + 水费 + 电费
```

### 缴费状态判定
```
if amountPaid >= totalAmount:
    status = "已缴"
elif amountPaid > 0:
    status = "部分缴纳"
else:
    status = "待缴"
```

## API 端点

### 租户管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/tenants` | 获取所有租户记录 |
| POST | `/api/tenants` | 创建或更新租户记录 |
| PUT | `/api/tenants/:id` | 更新指定 ID 的租户记录 |
| DELETE | `/api/tenants/:id` | 删除指定 ID 的租户记录 |
| GET | `/api/tenants/room/:room_number` | 根据房间号获取租户 |
| DELETE | `/api/tenants/room/:room_number` | 根据房间号删除租户 |
| DELETE | `/api/tenants` | 清空所有租户记录 |

### 财务统计

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/income-summary` | 获取收入汇总信息 |
| GET | `/api/income-summary?date=YYYY-MM` | 按月份筛选收入汇总 |

### Excel 导入

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/excel/import` | 从 Excel 文件导入租户数据 |

### CORS 配置
所有 API 均启用 CORS，允许所有来源访问。

## 前端数据流

### 数据源策略
1. **优先 Firestore**: 当 `firebaseConfig` 配置正确时使用 Firebase Firestore
2. **本地回退**: 配置缺失时使用 `localStorage`，键名 `tenants_mock_db`

### 核心 Hook: useTenants
```typescript
function useTenants() {
  const [records, setRecords] = useState<TenantRecord[]>([]);
  const [loading, setLoading] = useState(true);

  // 自动加载数据
  // 每 30 秒轮询刷新数据

  return { records, loading, updateRecord, addRecord, refreshRecords };
}
```

### 主要业务操作
1. **新建/编辑租客**: `openEdit` -> `handleFormSubmit` -> 创建或更新当月账单
2. **抄表录入**: `handleMeterSubmit` -> 计算水电费用 -> 新增或更新记录
3. **收款登记**: `handlePaySubmit` -> 累加 `amountPaid` -> 更新 `status`
4. **历史查询**: 按房号筛选，按日期降序展示

## AI 智能催款功能

### 配置
- SDK: `@google/genai`
- 模型: `gemini-3-flash-preview`
- API Key: 环境变量 `API_KEY` 或 `GEMINI_API_KEY`

### 提示词生成
根据租客姓名、房号、账期、未付金额生成催款文案。

## 数据库 Schema

### tenants 表
```sql
CREATE TABLE tenants (
  id TEXT PRIMARY KEY,
  room_number TEXT NOT NULL,
  name TEXT NOT NULL,
  phone TEXT,
  id_card TEXT,
  check_in_date TEXT,
  deposit REAL,
  rent_amount REAL NOT NULL,
  water_reading REAL NOT NULL DEFAULT 0,
  electricity_reading REAL NOT NULL DEFAULT 0,
  water_bill REAL NOT NULL DEFAULT 0,
  electricity_bill REAL NOT NULL DEFAULT 0,
  total_amount REAL NOT NULL DEFAULT 0,
  amount_paid REAL NOT NULL DEFAULT 0,
  rent_cycle TEXT DEFAULT '月度',
  utility_cycle TEXT DEFAULT '月度',
  status TEXT NOT NULL DEFAULT '待缴',
  date TEXT NOT NULL,
  recorded_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  monthly_income REAL NOT NULL DEFAULT 0,
  annual_income REAL NOT NULL DEFAULT 0,
  water_elec_income REAL NOT NULL DEFAULT 0,
  monthly_water_elec_income REAL NOT NULL DEFAULT 0,
  annual_water_elec_income REAL NOT NULL DEFAULT 0
);

CREATE INDEX idx_room_number ON tenants(room_number);
CREATE INDEX idx_date ON tenants(date);
CREATE INDEX idx_status ON tenants(status);
```

## 开发指南

### 启动后端
```bash
cd backend
go build -o bin/rentadmin.exe
./bin/rentadmin.exe
# 服务运行在 http://localhost:8080
```

### 启动前端
```bash
cd frontend
npm install
npm run dev
# 开发服务器运行在 http://localhost:5173
```

### Docker 部署
```bash
docker-compose up --build
```

## 相关文件

### 前端
- [index.tsx](frontend/index.tsx) - 主 React 组件，包含所有 UI 逻辑
- [types.ts](frontend/types.ts) - TypeScript 类型定义
- [utils.ts](frontend/utils.ts) - API 调用工具函数
- [constants.ts](frontend/constants.ts) - 常量和初始数据
- [API_DOC.md](frontend/API_DOC.md) - 前端 API 文档

### 后端
- [main.go](backend/main.go) - 程序入口
- [tenant.go](backend/src/models/tenant.go) - 数据模型
- [db.go](backend/src/database/db.go) - 数据库连接
- [migrations.go](backend/src/database/migrations.go) - 数据库迁移
- [tenant_handler.go](backend/src/handlers/tenant_handler.go) - HTTP 处理器
- [excel_handler.go](backend/src/handlers/excel_handler.go) - Excel 处理器
- [router.go](backend/src/routes/router.go) - 路由配置
- [excel_import.go](backend/src/services/excel_import.go) - Excel 导入服务
- [income_calculator.go](backend/src/utils/income_calculator.go) - 收入计算

---

# 代理体系（Agent System）

本仓库使用 **AGENTS.md 分层上下文** 来组织多个专家子代理。
不同子目录下的 `AGENTS.md` 会在 agent 进入该作用域时自动激活。

## 总代理（Orchestrator Agent）

**作用域**：仓库根目录（当任务跨越多端、需要协调时）。

**职责**：
1. **任务分诊**：根据用户请求关键字判断该路由到哪个子代理。
2. **跨端契约同步**：当一次需求同时涉及后端、前端、小程序时，先确认数据契约（字段名、状态码、错误约定），再按"后端 → 前端/小程序"顺序推进。
3. **冲突协调**：若三端代理给出的字段命名、状态码不一致，以 [`backend/src/models/`](backend/src/models/) 为唯一真源。
4. **质量把关**：完成功能后，必要时调用"代码审查代理"做最后一道检查。

### 路由规则

| 用户关键词 / 改动位置 | 路由到 |
|---------------------|--------|
| Go、handler、router、SQL、迁移、`backend/**` | **后端子代理** ([backend/AGENTS.md](backend/AGENTS.md)) |
| React、tsx、css、Vite、`frontend/**` | **前端子代理** ([frontend/AGENTS.md](frontend/AGENTS.md)) |
| 小程序、wxml、wxss、`miniprogram/**` | **小程序子代理** ([miniprogram/AGENTS.md](miniprogram/AGENTS.md)) |
| "review、审查、检查、重构建议、安全审计" | **代码审查代理**（下文） |
| 跨端功能、新业务流、端到端调试 | **总代理**（本节） |

### 跨端任务推进模板

```
1. 明确需求：用户问"…"，涉及哪几端？
2. 设计契约：在 backend/src/models/ 落地字段（数据库 + JSON tag）。
3. 后端实现：handler、router、migration、go build 通过。
4. 同步前端：types.ts、utils.ts、UI 调用。
5. 同步小程序：services/*.js、页面 setData、WXML 显示。
6. 自检：调用 code-review 代理对照清单审一遍。
```

---

## 代码审查子代理（Code Review Agent）

**作用域**：全局（不依赖具体目录，由总代理或用户显式调用）。

**职责**：以 senior engineer 视角审视当前 git 变更或指定文件，输出可执行的改进建议。

### 审查清单

#### 1. 正确性
- [ ] 边界条件：空值、零、负数、超长字符串、并发
- [ ] 错误处理：error 是否吞掉？panic 是否会扩散？
- [ ] 资源释放：`defer rows.Close()`、文件句柄、goroutine 泄漏
- [ ] SQL 注入：所有参数走 `?` 占位符
- [ ] 时间区：UTC 存储，展示层转本地

#### 2. 契约一致性
- [ ] 后端 JSON tag ↔ 前端 `types.ts` ↔ 小程序字段名三方一致
- [ ] HTTP 状态码语义正确（200/400/401/403/404/500）
- [ ] 枚举值（PaymentStatus、PaymentCycle）三端取值一致

#### 3. SOLID & 代码质量
- [ ] 单一职责：handler 不应直接操作复杂业务，应抽到 service
- [ ] 重复代码：相同 SQL 扫描逻辑应封装
- [ ] 命名：函数名表达意图，禁止 `data1`、`tmp`、`xxx`
- [ ] 函数长度：handler ≤ 60 行，超长必须拆分
- [ ] 魔法数字：水电单价等用常量

#### 4. 安全
- [ ] 鉴权：每个 `/api/mp/*` 必经 `requireOpenID`
- [ ] 越权：检查请求资源的归属（openid ↔ 房号 ↔ 账单 id）
- [ ] 敏感信息：身份证、手机号不写入日志
- [ ] CORS：生产环境不允许 `*`

#### 5. 性能
- [ ] N+1 查询：列表接口避免循环里查数据库
- [ ] 索引：高频查询字段（room_number、date、status、openid）必须有索引
- [ ] 轮询频率：前端 30s 一次合理，小程序按需调用

#### 6. UX / 可访问性
- [ ] 加载状态、错误提示是否完整
- [ ] 表单校验失败有明确文案
- [ ] 移动端 UI 不溢出
- [ ] 金额单位（元/分）转换不出错

### 输出格式

```
## 审查摘要
- 严重问题：N 个
- 一般建议：N 个
- 加分项：N 个

## 严重问题
### [文件:行号] 问题标题
**问题**：…
**风险**：…
**建议**：（含具体代码示例）

## 一般建议
…
```

---

## 子代理一览

| 代理 | 文件 | 自动激活条件 |
|------|------|------------|
| 后端 | [backend/AGENTS.md](backend/AGENTS.md) | 工作在 `backend/` 下 |
| 前端 | [frontend/AGENTS.md](frontend/AGENTS.md) | 工作在 `frontend/` 下 |
| 小程序 | [miniprogram/AGENTS.md](miniprogram/AGENTS.md) | 工作在 `miniprogram/` 下 |
| 代码审查 | 本文件 § 代码审查子代理 | 用户显式请求 review |
| 总代理 | 本文件 § 总代理 | 跨端任务或仓库根操作 |
