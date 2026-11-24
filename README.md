# Eino Go Demo

基于字节跳动开源框架 [Eino](https://github.com/cloudwego/eino) 的 Go 语言示例项目。项目通过多个递进章节演示 ChatModel、Agent、会话记忆、DeepAgent、Middleware、Callback/Trace，以及 Interrupt/Resume 等能力。

## 技术栈

- Go 1.26.10
- Eino 0.9.23
- Eino ADK（Agent Development Kit）
- 阿里云百炼 OpenAI 兼容接口
- Redis：会话记忆、Checkpoint 持久化
- CozeLoop：链路追踪与可观测性
- 本地文件后端：文件读取、写入、搜索和命令执行

## 项目结构

```text
eino-gfdemo/
├── 01_ChatModel/                 # ChatModel 基础调用与流式输出
├── 02_ChatModelAgent/            # ADK 单 Agent 与多轮对话
├── 03_Memory/                    # 基于 Redis 的会话历史持久化
├── 04_DeepAgent/                 # DeepAgent、本地文件后端与工具调用
│   ├── main.go                   # 文件系统 Agent 交互示例
│   └── read_analyze/main.go      # CSV 文件读取与数据分析示例
├── 05_Middleware/                # Middleware、工具错误转换与限流重试
├── 06_Callback与Trace/            # Callback 与 CozeLoop 链路追踪
├── 07_Interrupt与Resume/          # 人工介入、中断恢复与审批流程
│   ├── eino-redis-hitl/          # 首次中断后审批恢复示例
│   └── eino-redis-hitl_v2/       # 循环对话中的审批恢复示例
├── data/sales.csv                # DeepAgent 读取分析的数据文件
├── pkg/
│   ├── model_create.go           # 公共 ChatModel 创建函数
│   ├── project_root.go           # 项目根目录解析
│   └── redis_store.go            # Redis Checkpoint/会话存储实现
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

演示跨进程持久化会话历史：

- 将消息序列保存到 Redis。
- 根据固定 Session ID 加载历史。
- 新消息和 Agent 回复都会追加到历史并再次保存。
- 本示例中的会话数据不设置过期时间，业务中可按需求设置 TTL。

运行：

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

- 使用 `backend.Write` 创建或覆盖 [data/sales.csv](data/sales.csv)。
- 使用真实绝对路径调用 DeepAgent。
- Windows 路径转换为正斜杠后传给 Agent。
- 限制 Agent 使用只读工具，避免无意义的文件修改。
- 读取 CSV 并指出营收最高的产品。

运行：

```powershell
go run ./04_DeepAgent/read_analyze
```

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

`pkg.NewChatModel(ctx)` 会：

- 自动加载项目根目录的 `.env`。
- 校验三个必需环境变量。
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

## 常见运行方式

运行任意独立示例：

```powershell
go run ./目录名
```

运行目录名包含中文或使用子目录时建议加引号：

```powershell
go run './06_Callback与Trace'
go run './07_Interrupt与Resume/eino-redis-hitl_v2'
```

编译整个模块：

```powershell
go build ./...
```

格式化代码：

```powershell
gofmt -w ./
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

本地文件后端直接读取操作系统路径，不会自动把 `/data/sales.csv` 映射为项目目录。应使用项目绝对路径，例如：

```text
/eino-gfdemo/data/sales.csv
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
