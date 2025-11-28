# GoStacker

GoStacker 是一个完整的即时通讯（IM）后端，提供消息发送、聊天室/群组管理、连接管理、消息持久化与实时推送等能力。项目既支持通过独立 Dispatcher 进行网关转发（Gateway 模式），也支持单进程直接投递（Standalone 模式）。仓库重点在推送引擎的设计与实现（低延迟、高并发与可靠投递），底层使用 Redis 作为队列/回退存储，MySQL 作为业务数据持久化，使用 goroutines + channels 实现并发调度，并通过 `gorilla/websocket` 提供实时通道。

---

**主要特性**

- 完整 IM 后端能力：支持点对点消息、群组/聊天室、消息构建与持久化（MySQL 可选）以及业务侧发送逻辑。
- 推送引擎为核心重点：支持 Gateway 模式与 Standalone 模式，部署灵活，针对高并发优化的低延迟投递路径。
- 每用户独立连接持有（ConnectionHolder），通过单独 writerLoop 序列化 websocket 写入，保证并发写安全与顺序性。
- 离线/等待队列（Redis）：`offline:push:{user}` 与 `wait:push:{user}`，结合 `wait:push:set` 做有序重试与回灌，确保消息可靠性与至少一次投递语义。
- Dispatcher 支持 worker pool（可配置）与任务拆分（按用户分批，默认 100 人一组）以实现削峰与负载均衡。
- 可配置的流控与短超时策略（100-200ms）避免生产方阻塞，并把高峰流量降级到 Redis 持久化队列以保证系统可用性。
- 使用 `zap` 做结构化日志记录，并具备 Graceful Shutdown 与后台重试监听器以提高稳定性。

---

**仓库结构（简要）**

- `main.go`：程序入口，初始化配置、日志、DB、Redis、Dispatcher 等。
- `pkg/push/`：推送核心实现（dispatcher、gateway dispatch、standalone dispatch、连接管理、离线回灌等）。
- `internal/gateway/`：网关相关组件与路由（网关向最终客户端转发逻辑）。
- `pkg/db/redis`：Redis 封装（带重试封装的常用操作）。
- `pkg/logger`：zap 日志初始化与中间件。
- `internal/send`、`internal/chat`：消息构造及发送侧业务逻辑。

---

**推送子系统说明（重点）**

推送子系统位于 `pkg/push`，包含以下主要角色：

- Dispatcher：接收上层应用的 PushMessage，将目标用户按策略拆分为子任务并入队（Gateway 模式会按所属 gateway 聚合用户并通过内部 WS 发送给对应 gateway）。
- GatewayDispatcher：在 Gateway 模式下负责把消息路由到正确的 gateway；若 gateway 不在线则降级到用户离线队列。[gateway仓库](https://github.com/ANormalDD/DistrributePusher_Gateway)
- Standalone Dispatcher：在单进程模式下直接尝试把消息入用户内存发送队列（非阻塞入队），失败时写入 Redis 的 wait/offline 队列。
- ConnectionHolder + writerLoop：每个用户有一个 ConnectionHolder（包含 websocket.Conn、带缓冲的 sendCh），`writerLoop` 串行化写入以避免 websocket 并发写冲突。
- 离线/等待队列：
  - `offline:push:{user}`：长期离线消息，用户登录时由 `PushOfflineMessages` 批量拉取并回灌。
  - `wait:push:{user}` + `wait:push:set`：短期等待队列，用于削峰与异步重试，后台 `ListeningWaitQueue` 会遍历 `wait:push:set` 并尝试把队列消息重新入队。

关键设计要点：

- 非阻塞入队与短超时：入队与写操作设置短超时（常见 100–200ms），避免生产者被阻塞，突发流量被降级到 Redis。 
- 批量拆分：对于大规模广播消息，Dispatcher 将目标用户切分为小批次（默认 100 人一组）降低单次调度压力。
- 重试与回退：写失败时有重试机制；关键场景把消息持久化到 Redis，保证“至少一次”投递语义。
- 可观测性：对队列满、Redis 操作失败、网关不可用等关键事件进行日志记录以便排查和报警。

---

**快速开始（Windows PowerShell）**

1. 设置配置文件（示例 `config.yaml` 在项目根）：编辑数据库、Redis、Dispatcher 等配置项。

2. 本地运行（开发模式）：

```powershell
# 在项目根目录执行
go run main.go
```

或构建并运行：

```powershell
go build -o gostacker.exe
.
./gostacker.exe
```

3. 运行前请确保：

- 已启动 Redis 实例并在 `config.yaml` 中配置正确地址。
- 已配置 MySQL（若需要持久化聊天记录/其他数据）。

---

**配置亮点**

- `DispatcherConfig`：可配置 `GatewayWorkerCount`、`GatewayQueueSize`、`SendChannelSize` 等，用于调优并发与内存使用。
- `PushMod`：选择 `standalone` 或 `gateway` 模式。
- `GroupCacheConfig`：分组消息写回（flusher）相关设置（间隔、批次大小）。

具体配置项请参阅项目中的 `config` 包与 `config.yaml` 示例。

---

**测试与调优建议**

- 使用并发 WebSocket 客户端模拟器（自制或现成工具）验证 TPS、P95 延迟和连接吞吐量。
- 观测 Redis 列表长度、`wait:push:set` 大小与 gateway dispatch queue 长度来判断系统削峰是否充分。
- 调整 `SendChannelSize` 与 dispatcher worker 数量以取得最优延迟/吞吐平衡。

---

**部署建议**

- Gateway 模式：前端部署多个 Gateway 实例（负责外部 websocket 连接），并把推送服务部署为独立 Dispatcher 服务，Dispatcher 将消息发送到对应 Gateway。这样可以水平扩展外部连接与推送单元。
- Standalone 模式：适合单机或容器化部署的简单场景，适合小规模或快速验证环境。

---

**TODO LIST**
- 使用监控（Prometheus + Grafana）采集关键指标：队列长度、入队延迟、写入失败率、Redis 操作错误率等。
- 对于redis无法通信或者挂掉时的本地缓存
- Docker部署
- 推送消息全链路追踪
- 支持文件上传，分布式存储


