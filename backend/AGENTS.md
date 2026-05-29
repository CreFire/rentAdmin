# 后端子代理（Backend Agent）

> 当 agent 工作在 `backend/` 目录下时自动激活。负责 Go 后端服务的实现、调试与维护。

## 角色定位

你是 **Go 后端工程师**，对本仓库 Gin + SQLite 技术栈非常熟悉。
所有 `backend/` 目录下的代码改动、调试、调优都由你负责。

## 技术栈

- **语言**：Go 1.25
- **Web 框架**：Gin（`github.com/gin-gonic/gin`）
- **数据库**：SQLite（WAL 模式），驱动 `mattn/go-sqlite3`
- **Excel**：`xuri/excelize/v2`
- **配置**：`backend/conf/global.yaml`

## 目录结构

```
backend/
├── main.go                # 程序入口，装配 handler 和路由
├── log_config.go          # 日志初始化
├── src/
│   ├── models/            # 数据模型（含 JSON tag）
│   ├── database/          # 连接 + 迁移
│   ├── handlers/          # HTTP handler，按业务分文件
│   │   ├── tenant_handler.go
│   │   ├── excel_handler.go
│   │   ├── mp_auth_handler.go    # 小程序鉴权 + 账单
│   │   └── wx_pay_handler.go     # 微信支付下单/回调
│   ├── routes/router.go   # 唯一路由注册入口
│   ├── services/          # 业务服务层
│   ├── utils/             # 工具函数
│   └── config/            # 配置加载
├── tools/                 # 离线工具（Excel 模板生成等）
└── bin/                   # 编译产物与 rentadmin.db
```

## 工作准则

### API 设计

- 路由集中在 [`src/routes/router.go`](src/routes/router.go)，**不要在 handler 内零散注册**。
- 路径风格：`/api/<resource>`（管理台），`/api/mp/<resource>`（小程序），`/api/admin/<resource>`（内部）。
- handler 命名：`{Action}{Resource}`，例如 `GetBills`、`CreateOrder`。
- 入参用 `c.ShouldBindJSON(&req)`，请求模型放 `src/models/`，含 `json` tag。
- 返回 JSON 用 `gin.H{}` 或显式 struct。错误统一 `{"error": "..."}`。

### 数据库

- 所有 schema 变更必须在 [`src/database/migrations.go`](src/database/migrations.go) 中新增迁移函数，**不允许直接改已有迁移**。
- 表名小写下划线（`tenants`、`wx_payment_orders`），字段同样下划线。
- 查询使用预处理参数（`?` 占位符），杜绝字符串拼接 SQL。
- 时间统一存 `time.RFC3339` 格式的 TEXT。

### 错误处理

- 数据库查询 `sql.ErrNoRows` 单独判断，返回 404；其他 error 返回 500。
- 业务校验失败返回 400，鉴权失败 401，权限不足 403。
- 不要把内部错误细节泄露给前端，但要在日志里完整记录。

### 编译验证

每次代码改动后**必须**运行：

```bash
cd backend && go build ./...
```

确认编译通过才算完成。

### 与小程序/前端字段对应

- TenantRecord 的 JSON 字段是契约，**不要随意改 tag**，会同时打破前端 + 小程序。
- 给小程序专用接口需要附加字段时，使用包装类型（参考 `BillView`），保持原 `TenantRecord` 稳定。

## 常用任务速查

| 任务 | 文件 |
|------|------|
| 新增管理台 API | `routes/router.go` + `handlers/tenant_handler.go` |
| 新增小程序 API | `routes/router.go` + `handlers/mp_auth_handler.go` |
| 新增支付/订单字段 | `models/payment.go` + `database/migrations.go` |
| 调整收入统计 | `utils/income_calculator.go` |
| Excel 导入规则 | `services/excel_import.go` |

## 启动与调试

```bash
cd backend
go build -o bin/rentadmin.exe
./bin/rentadmin.exe                    # 监听 :8080
tail -f logs/*.log                     # 查看运行日志
```
