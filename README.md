# Eino Go Demo

基于字节跳动开源框架 [Eino](https://github.com/cloudwego/eino) 的 Go 语言示例项目。项目通过多个递进章节演示 ChatModel、Agent、会话记忆、DeepAgent、Middleware、Callback/Trace，以及 Interrupt/Resume 等能力。

## 技术栈

- Go 1.26.10
- Eino 0.9.23
- Eino ADK（Agent Development Kit）
- 阿里云百炼 OpenAI 兼容接口
- Redis：会话缓存与 Checkpoint 持久化
- MySQL：会话消息全量持久化
- CozeLoop：链路追踪与可观测性
- 本地文件后端：文件读取、写入、搜索和命令执行
- Gin：SSE 流式聊天 Web UI

## 项目结构

```text
eino-gfdemo/
├── 01_ChatModel/                 # ChatModel 基础调用与流式输出
│   ├── main.go                   # ChatModel 示例选择菜单
│   ├── ChatModelDemo/            # 普通调用与流式调用示例
│   ├── PromptDemo/               # Prompt 模板与思维链示例
│   └── AgenticModel/             # Agentic Model 示例
├── 02_ChatModelAgent/            # ADK 单 Agent 与多轮对话
├── 03_Memory/                    # Redis、Redis + MySQL 会话存储
│   ├── main.go                   # 会话存储示例选择菜单
│   └── MemoryDemo/
│       ├── redis存储会话.go
│       └── redis+mysql存储会话.go
├── 04_DeepAgent/                 # DeepAgent、本地文件后端与工具调用
│   ├── main.go                   # 文件系统 Agent 交互示例
│   └── read_analyze/main.go      # CSV 文件读取与数据分析示例
├── 05_Middleware/                # Middleware、工具错误转换与限流重试
├── 06_Callback与Trace/            # Callback 与 CozeLoop 链路追踪
├── 07_Interrupt与Resume/          # 人工介入、中断恢复与审批流程
│   ├── eino-redis-hitl/          # 首次中断后审批恢复示例
│   └── eino-redis-hitl_v2/       # 循环对话中的审批恢复示例
├── resources/data/sales.csv      # DeepAgent 读取分析的数据文件
├── pkg/
│   ├── chat_store.go             # Redis + MySQL 会话消息存储
│   ├── cmd_template.go           # 示例菜单公共方法
│   ├── model_create.go           # 公共 ChatModel 创建函数
│   ├── project_root.go           # 项目根目录解析
│   ├── print_use_token.go        # Token 使用量输出
│   └── redis_store.go            # Redis Checkpoint/会话存储实现
├── ui/
│   ├── main.go                   # Gin + SSE 聊天服务
│   └── templates/index.html      # 聊天页面
├── go.mod
├── go.sum
└── .gitignore
```

## 环境准备

### 1. 安装 Go

建议使用与 `go.mod` 一致的 Go 1.26 或更新版本。

```powershell
# 验证版本
go version

# 安装依赖
go mod download
```

### 2. 配置阿里云百炼

在项目根目录创建 `.env` 文件：

```env
DASHSCOPE_API_KEY=你的APIKey
DASHSCOPE_MODE=模型名称
DASHSCOPE_BASE_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
```

常见模型名可在阿里云百炼控制台查看。

使用 Redis 或 CozeLoop 的示例还需要配置：

```env
COZELOOP_API_TOKEN=你的CozeLoopToken
COZELOOP_WORKSPACE_ID=你的CozeLoop工作区ID
PROJECT_ROOT=\eino-gfdemo
```

Redis + MySQL 会话存储示例还需要配置：

```env
REDIS_ADDR=127.0.0.1:6379
MYSQL_DSN=user:password@tcp(127.0.0.1:3306)/eino_demo?charset=utf8mb4&parseTime=True&loc=Local
```

> `.env` 已被 `.gitignore` 排除，不应提交真实密钥。

### 3. 启动 Redis

部分示例默认连接 `127.0.0.1:6379`，确保本机 Redis 已启动：

```powershell
redis-server
redis-cli ping
```

## 功能说明

### 01 - ChatModel

入口：[01_ChatModel/main.go](01_ChatModel/main.go)

演示最基础的模型调用能力：

- 使用阿里云百炼 OpenAI 兼容接口创建 ChatModel。
- 构造 system/user 消息。
- 通过 `Stream` 接收流式响应。
- 使用 `BindTools` 绑定工具描述，并演示模型返回工具调用参数。
- 展示 Temperature、TopP、MaxTokens 等生成参数。
- 使用 `prompt.FromMessages` 和 `{变量名}` 占位符构造可复用的 Prompt 模板。
- 根据风格和领域动态生成 System Prompt。
- 从结构体准备模板变量，并展示逐消息格式化结果。
- 使用 System Prompt 和用户问题构造思维链推理流程。

Prompt 示例位于 [01_ChatModel/PromptDemo](01_ChatModel/PromptDemo)：

- [prompt基础.go](01_ChatModel/PromptDemo/prompt基础.go)：基础 FString 模板，演示 `{role}` 和 `{input}` 变量替换。
- [动态模版.go](01_ChatModel/PromptDemo/动态模版.go)：根据专业、轻松、友好或正式风格动态生成 System Prompt。
- [结构体模版.go](01_ChatModel/PromptDemo/结构体模版.go)：将 `UserProfile` 字段映射为模板变量。
- [思维链.go](01_ChatModel/PromptDemo/思维链.go)：组合理解、分析、计算、验证和结论步骤，并调用 ChatModel 流式输出。

运行：

```powershell
go run ./01_ChatModel
```

### 02 - ChatModelAgent

入口：[02_ChatModelAgent/main.go](02_ChatModelAgent/main.go)

演示 ADK 中的 ChatModelAgent：

- 创建 Agent 和 Runner。
- 从标准输入读取用户问题。
- 通过 `Run` 接收流式事件。
- 在进程内维护历史消息，实现多轮对话。
- 输入 `exit` 退出。

运行：

```powershell
go run ./02_ChatModelAgent
```

### 03 - Redis 会话记忆

入口：[03_Memory/main.go](03_Memory/main.go)

入口提供两种会话存储示例：

- 选项 1：将会话历史保存在 Redis Checkpoint Store 中，按固定 Session ID 加载和保存多轮对话。
- 选项 2：将消息全量写入 MySQL，并在 Redis 中缓存最近 10 条消息，缓存有效期为 24 小时。
- Redis + MySQL 示例包含天气查询工具；持久化历史保留用户消息和最终助手答复，不将工具调用中间态作为下一轮输入。
- Redis + MySQL 示例配置摘要中间件，在上下文消息达到 25 条时触发摘要，并将摘要写入 MySQL；退出时会显示最近一次摘要。

Redis + MySQL 示例需要配置 `REDIS_ADDR` 和 `MYSQL_DSN`，并预先创建 MySQL 数据库；程序会自动迁移消息表和摘要表。

运行并在菜单中选择示例：

```powershell
go run ./03_Memory
```

### 04 - DeepAgent 与本地文件系统

入口：[04_DeepAgent/main.go](04_DeepAgent/main.go)

演示带文件工具和命令执行能力的 DeepAgent：

- 创建本地文件后端 `local.NewBackend`。
- 使用绝对项目根目录解析文件和目录请求。
- 在 Agent 中配置文件系统工具和 StreamingShell。
- 限制最大迭代次数。
- 支持流式多轮对话。

运行：

```powershell
go run ./04_DeepAgent
```

CSV 分析示例：[04_DeepAgent/read_analyze/main.go](04_DeepAgent/read_analyze/main.go)

该示例演示：

- 使用 `backend.Write` 创建或覆盖 [resources/data/sales.csv](resources/data/sales.csv)。
- 使用真实绝对路径调用 DeepAgent。
- Windows 路径转换为正斜杠后传给 Agent。
- 限制 Agent 使用只读工具，避免无意义的文件修改。
- 读取 CSV 并指出营收最高的产品。

运行：

```powershell
go run ./04_DeepAgent/read_analyze
```

### Web UI

入口：[ui/main.go](ui/main.go)

提供 Gin 聊天页面和 SSE 流式接口。服务默认监听 `8080`，可通过 `PORT` 环境变量修改端口。

```powershell
go run ./ui
```

启动后访问 `http://localhost:8080`。调用模型仍需配置阿里云百炼环境变量。

### 05 - Middleware

入口：[05_Middleware/main.go](05_Middleware/main.go)

演示 Agent 的可扩展执行能力：

- 自定义工具错误中间件，将工具异常转换为可供模型继续处理的文本。
- 配置模型重试策略，针对 429、Too Many Requests 和 qpm limit 进行重试。
- 支持本地文件后端和 StreamingShell。
- 包含流式对话和工具调用的交互循环。
- 代码中保留了摘要和缩减 Middleware 的概念配置，可用于长上下文场景。

运行：

```powershell
go run ./05_Middleware
```

### 06 - Callback 与 CozeLoop Trace

入口：[06_Callback与Trace/main.go](06_Callback与Trace/main.go)

演示调用生命周期回调和外部可观测性平台：

- 使用 `callbacks.NewHandlerBuilder` 注册开始、结束和异常回调。
- 注册全局 Callback 处理器。
- 使用 CozeLoop Handler 将执行链路发送到工作区。
- 与 05 章节类似，包含 DeepAgent、文件工具、重试和流式交互。

运行：

```powershell
go run ./06_Callback与Trace
```

未配置 `COZELOOP_API_TOKEN` 或 `COZELOOP_WORKSPACE_ID` 时，CozeLoop 客户端创建可能失败。

### 07 - Interrupt 与 Resume

入口：[07_Interrupt与Resume/main.go](07_Interrupt与Resume/main.go)

演示基础目录入口和人工审批流程的组织方式。

#### 单次审批示例

入口：[07_Interrupt与Resume/eino-redis-hitl/main.go](07_Interrupt与Resume/eino-redis-hitl/main.go)

- 定义可中断的 `DeleteFile` 工具。
- 首次调用时通过 `compose.Interrupt` 暂停。
- 将中断状态和 Checkpoint 保存到 Redis。
- 用户输入 `y/n` 后通过 `ResumeWithParams` 继续执行。
- 同意时返回已删除；拒绝时返回拒绝结果。

运行：

```powershell
go run ./07_Interrupt与Resume/eino-redis-hitl
```

#### 循环审批示例

入口：[07_Interrupt与Resume/eino-redis-hitl_v2/main.go](07_Interrupt与Resume/eino-redis-hitl_v2/main.go)

- 在交互循环中处理连续对话和审批。
- 使用固定 Checkpoint ID 保存和恢复执行状态。
- 输入 `exit` 退出；删除类请求会触发人工审批。
- 适合演示长会话中断与恢复。

运行：

```powershell
go run ./07_Interrupt与Resume/eino-redis-hitl_v2
```

## 公共组件

### ChatModel 工厂

[pkg/model_create.go](pkg/model_create.go)

`pkg` 包初始化时从当前工作目录加载 `.env` 并校验以下必需变量；`pkg.NewChatModel(ctx)` 使用这些配置创建模型：

- `DASHSCOPE_API_KEY`
- `DASHSCOPE_MODE`
- `DASHSCOPE_BASE_URL`

模型工厂会：

- 创建阿里云百炼兼容 ChatModel。
- 配置超时、Temperature、TopP 和最大 Token 数量。

### 项目根目录

[pkg/project_root.go](pkg/project_root.go)

`pkg.GetProjectRoot()` 优先读取 `PROJECT_ROOT`，未配置时使用当前工作目录，并转换为绝对路径。

### Redis Checkpoint 存储

[pkg/redis_store.go](pkg/redis_store.go)

提供 `adk.CheckpointStore` 的 Redis 实现：

- `Set`：保存指定 Key 的中断状态。
- `Get`：读取中断状态并区分不存在与错误。
- `Delete`：清理 Checkpoint。
- `Save/Load`：保存或加载会话消息。

### Redis + MySQL 会话存储

[pkg/chat_store.go](pkg/chat_store.go)

- MySQL 保存完整会话消息。
- Redis List 使用 `RPush` 按时间顺序追加，读取结果无需反转；列表仅保留最近 10 条，缓存 TTL 为 24 小时。
- Redis 缓存未命中时从 MySQL 读取最近消息并回填缓存。
- `SaveSummary` 和 `GetLatestSummary` 用于持久化和读取会话摘要；缓存命中与回源时都会将最新摘要放入 Agent 历史上下文。
- `Close` 释放 MySQL 连接池和 Redis 客户端。

## 常见运行方式

运行路径为 ASCII 的独立示例：

```powershell
go run ./01_ChatModel
go run ./ui
```

目录名包含中文时，直接指定 Go 源文件路径，避免 Go 将中文目录解析为 package import path：

```powershell
go run './06_Callback与Trace/main.go'
go run './07_Interrupt与Resume/eino-redis-hitl_v2/main.go'
```

单独编译一个包：

```powershell
go build ./ui
go build ./03_Memory
```

由于部分目录名包含中文，`go build ./...` 目前会因非法 package import path 失败；若需全模块构建，应先将这些目录重命名为 ASCII 名称。格式化单个 Go 文件：

```powershell
gofmt -w .\path\to\file.go
```

## 常见问题

### 缺少环境变量

检查 `.env` 是否位于项目根目录，并确认变量名为：

- `DASHSCOPE_API_KEY`
- `DASHSCOPE_MODE`
- `DASHSCOPE_BASE_URL`

### Redis 连接失败

确认 Redis 已启动，并检查：

```powershell
redis-cli ping
```

默认地址应为 `127.0.0.1:6379`。

### 读取文件失败

本地文件后端直接读取操作系统路径，不会自动把 `/resources/data/sales.csv` 映射为项目目录。应使用项目绝对路径，例如：

```text
/eino-gfdemo/resources/data/sales.csv
```

在 Windows 上可以将 `\\` 转换为 `/` 后传给模型和工具。

### 模型调用出现 429

通常为请求频率限制或配额问题，可：

- 调整请求频率。
- 使用 05 或 06 示例中的重试配置。
- 查看阿里云百炼控制台配额和当前模型状态。

### 中断恢复失败

需要确认：

- Redis 正在运行。
- 首次运行和恢复使用相同 Checkpoint ID。
- `ResumeWithParams` 使用的 Interrupt ID 与中断事件一致。
- 未手动删除对应 Redis Checkpoint。

## 安全提示

- 不要将 `.env`、API Key 或工作区 Token 提交到 Git。
- 删除、修改、写入文件等工具应始终进行人工审批或权限校验。
- 本项目中的 DELETE 示例目前主要演示流程，不应直接用于生产环境。
