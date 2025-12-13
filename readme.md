**项目简介**
 - **名称**: GoStacker — 一个轻量级、高性能的即时通讯 (IM) 后端服务。
 - **职责**: 提供连接网关、路由与发送、离线消息持久化、群组缓存写回、服务注册/发现与推送分发等 IM 后端能力。

**仓库定位**
 - **入口**: [main.go](main.go)（单进程整合启动）；另外提供子服务启动入口如 [cmd/gateway/main.go](cmd/gateway/main.go) 等用于拆分部署。

**目录结构概览**
 - **cmd/**: 各可独立运行的子服务入口，比如 `gateway`, `send`, `meta`, `flusher`。查看例如 [cmd/gateway/main.go](cmd/gateway/main.go) 。
 - **internal/**: 核心业务实现（gateway、send、meta）。
 - **pkg/**: 可复用底层模块（`bootstrap`, `config`, `db`, `logger`, `monitor`, `push` 等）。例如配置在 [pkg/config/config.go](pkg/config/config.go) 。
 - **config*.yaml**: 多个示例/环境配置文件（`config.yaml`, `config.gateway.yaml` 等）。

**整体架构与实现逻辑**
 - **启动与初始化**: 启动时通过 `pkg/config` 加载 YAML 配置，初始化日志、MySQL、Redis、监控等（参见 [pkg/bootstrap/bootstrap.go](pkg/bootstrap/bootstrap.go) 和 [main.go](main.go)）。
 - **模块划分**:
	 - **Gateway 层**（`cmd/gateway` + `internal/gateway`）：负责接收客户端连接（HTTP/WS）、连接注册到中心/路由、与中心服务通信、将消息下发给内部发送组件。
	 - **Send 层**（`cmd/send` + `internal/send`）：负责将消息投递到目标连接（网关）或持久化/离线写入 MySQL；包含发送回退、负载分发等机制。
	 - **Meta 层**（`cmd/meta` + `internal/meta`）：用户、群组元数据管理，群组缓存、群组消息写回（flusher）逻辑。
	 - **Flusher**（`cmd/flusher` + `internal/meta/chat/group/flusher.go`）：当启用 Group Cache 时，周期性将群组缓存写回 MySQL。
	 - **Push 分发**（`pkg/push`, `internal/push`）：同一进程或独立进程两种 Push 模式（`push_mod` 配置：`standalone` 或 `gateway`），负责调度与下发离线通知或广播。
 - **消息流（高层）**:
	 1. 客户端 -> Gateway（WS/HTTP）
	 2. Gateway 验证/鉴权 -> 将消息送到 Send Dispatcher 或通过 Center 转发
	 3. Send 层根据目标路由信息选择网关实例或走离线写入（MySQL/Redis）
	 4. 若启用缓存，Flusher 后台将 Redis 缓存批量写回 MySQL

**核心配置说明**
 - 配置入口: [pkg/config/config.go](pkg/config/config.go)，配置通过 `viper` 从 `config.yaml`（或 `--config` 指定文件）加载。
 - 重要配置项:
	 - **Port/Address/Name**: 服务监听地址/端口/实例名
	 - **PushMod**: 推送模式（`standalone` 或 `gateway`）
	 - **MySQLConfig/RedisConfig**: 数据库与缓存连接
	 - **GroupCacheConfig**: 群组缓存开关与写回参数（`enabled`、`flush_interval_seconds`、`batch_size`）
	 - **SendDispatcherConfig/GatewayDispatcherConfig**: 发送/网关分发线程池与队列大小

**构建与运行**
 - 依赖: 需要本地安装 `go`（建议 1.20+），以及可访问的 MySQL 与 Redis。依赖在 `go.mod` 中声明。
 - 常用构建命令:

```bash
# 在仓库根目录构建主可执行文件（整合版）
go build -o bin/gostacker main.go

# 构建并运行单一子服务（示例：gateway）
cd cmd/gateway
go build -o ../../bin/gateway main.go
./../../bin/gateway -config config.gateway.yaml
```

 - 直接运行（开发）:

```bash
go run main.go            # 使用根目录 config.yaml
go run ./cmd/gateway -config config.gateway.yaml
```

**运行示例（推荐开发步骤）**
 - 准备 `config.gateway.yaml`、`config.send.yaml`、`config.meta.yaml`，修改 MySQL/Redis 地址。
 - 启动网关: `go run ./cmd/gateway -config config.gateway.yaml`。
 - 启动 send: `go run ./cmd/send -config config.send.yaml`。
 - 启动 meta: `go run ./cmd/meta -config config.meta.yaml`（如需要）。

**日志与监控**
 - 日志: 使用 `pkg/logger`（zap）输出，配置在 `log` 段里。查看日志路径通过配置 `log.filename`。
 - 监控: 程序在启动时调用 `monitor.InitMonitor()` 暴露基础指标，可接入 Prometheus。查看 [pkg/monitor/monitor.go](pkg/monitor/monitor.go)。

**数据存储与缓存策略**
 - MySQL: 存储历史消息、用户、群组等持久化数据（参见 `model/*.sql`）。
 - Redis: 用作在线路由、群组缓存与中间队列，高并发场景下使用 Redis 批量/流水线操作以降低延迟。

**推送模式说明**
 - `push_mod = standalone`:
	 - 内置 Dispatcher 在当前进程处理推送，并启动 flusher/background flusher（如在 `main.go` 中所示）。
 - 非 `standalone`:
	 - 通过 `push.StartGatewayDispatcher` 启动针对 Gateway 的分发器，适用于分布式部署，推送逻辑与网关分离。

**开发与调试建议**
 - 本地测试可用 `config.gateway.yaml`、`config.send.yaml` 调试单模块。使用 `go run` 启动便于热重启与快速迭代。
 - 使用 `zap` 日志级别调整（config.log.level）以便在开发时打印更多调试信息。
 - 调试连接与路由问题时，可查看 Redis 中的在线路由键与 Center 注册信息（`internal/gateway/center_client`）。

**常见问题与注意事项**
 - 配置变更会被 `viper` 监听并自动加载，请注意生产环境配置一致性。
 - 关闭服务时会依次关闭 MySQL/Redis 连接并 flush 日志。
 - 当启用群组缓存并使用 flusher 时，请根据消息量调整 `batch_size` 与 `flush_interval_seconds`，避免一次性写入过大批次导致数据库压力峰值。

**扩展与部署建议**
 - 建议将 `gateway` 与 `send` 拆分部署，分别做水平扩展。Center 负责注册/发现，建议使用单独的注册中心服务或通过数据库/Redis 实现全局视图。
 - 使用容器化（Docker）部署，每个子服务一个容器，配合 Kubernetes 做自动伸缩并使用 ConfigMap 管理 YAML 配置。

**阅读源码的关键文件**
 - `main.go` — 程序入口，展示了初始化顺序与推送模式分支。[main.go](main.go)
 - `pkg/bootstrap/bootstrap.go` — 常用初始化封装与清理函数。[pkg/bootstrap/bootstrap.go](pkg/bootstrap/bootstrap.go)
 - `pkg/config/config.go` — 所有可配置项结构与加载逻辑。[pkg/config/config.go](pkg/config/config.go)
 - `internal/send` — 发送相关实现（分发、路由、持久化）。
 - `internal/gateway` — 网关实现（中心注册、连接管理、HTTP/WS 路由）。


