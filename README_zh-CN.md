# GoStacker

![Go Version](https://img.shields.io/github/go-mod/go-version/ANormalDD/GoStacker)
[![Go Report Card](https://goreportcard.com/badge/github.com/ANormalDD/GoStacker)](https://goreportcard.com/report/github.com/ANormalDD/GoStacker)
![License](https://img.shields.io/github/license/ANormalDD/GoStacker)

> **GoStacker** 是一个高性能、可扩展的分布式即时通讯 (IM) 系统。
> 它采用微服务架构设计，支持**多网关负载均衡**、**消息可靠投递**与**写扩散优化**，旨在深入理解分布式 IM 系统的架构设计与实现细节。

## 技术栈

- **语言**: Go 1.25+
- **Web 框架**: [Gin](https://github.com/gin-gonic/gin)
- **WebSocket**: [gorilla/websocket](https://github.com/gorilla/websocket)
- **数据库**: MySQL (via [sqlx](https://github.com/jmoiron/sqlx) + [sqlhooks](https://github.com/qustavo/sqlhooks))
- **缓存/消息队列**: Redis (go-redis v9)，使用 Redis Stream 作为消息队列
- **认证**: JWT ([golang-jwt/jwt](https://github.com/golang-jwt/jwt))
- **ID 生成**: [Snowflake](https://github.com/bwmarrin/snowflake) 分布式 ID
- **配置管理**: [Viper](https://github.com/spf13/viper) + YAML
- **日志**: [Zap](https://go.uber.org/zap) + [Lumberjack](https://github.com/natefinch/lumberjack) 日志轮转
- **监控**: [Prometheus](https://github.com/prometheus/client_golang) 指标暴露 (`/metrics`)
- **密码加密**: golang.org/x/crypto (bcrypt)
- **本地缓存**: [Ristretto](https://github.com/dgraph-io/ristretto)

## 核心设计亮点

### 1. 微服务架构设计
拆分为 Gateway（长连接管理）、Send（消息路由）、Meta（用户/群组管理）、Registry（服务注册发现）四大服务，各服务独立部署、职责清晰。

### 2. 自研服务注册与动态负载均衡
基于 Redis ZSet 实现 Gateway 实例的实时负载排名，结合心跳健康检查与 TTL 自动过期，实现故障自动摘除与最低负载路由。

### 3. 高并发 WebSocket 连接管理
使用 `sync.Map` 存储连接映射，每连接配备独立发送队列与 Writer 协程，避免并发写冲突；支持连接迁移时未发送消息的自动排空与重投递。

### 4. 缓存写回机制（Write-Back）
群组成员数据写入 Redis 缓存并标脏（ZSet 时间戳排序），由后台 Flusher 服务定时批量刷盘至 MySQL，通过 Redis Pipeline 原子清除脏标记并设置短 TTL，兼顾读性能与数据一致性。

### 5. 消息可靠性保障
实现分片锁 Pending Task 管理器（64 分片 + atomic），追踪每条消息的推送完成状态；用户离线时自动触发 PushBack 机制将消息回退至 Send 服务重新路由。

### 6. 可观测性建设
自研滑动窗口 Monitor（环形缓冲区 + 异步插入），实时统计 API 平均延迟与成功率，并集成 Prometheus 暴露 `/metrics` 端点；使用 Zap 结构化日志 + Lumberjack 日志轮转。

### 7. 用户路由租约优化（Lease）
用户断连时不立即删除 User→Gateway 路由映射，而是将状态标记为 `disconnected` 并保留 TTL（可配置），用户短时间内重连时优先复用原 Gateway，避免重新分配带来的负载均衡抖动与连接迁移开销；Registry 和 Send 侧可配置 User→Gateway 本地缓存（TTL 短于 Redis），减少高频路由查询的网络往返。

## 架构概览

![architecture](structure.png)

系统由 **5 个微服务** 组成，各服务职责清晰、独立部署，支持水平扩展：

| 服务 | 默认端口 | 说明 |
|------|----------|------|
| **Meta Service** | 8082 | 用户/群组元数据管理，提供注册、登录、群组 CRUD 等 API |
| **Registry Service** | 8083 | 服务注册与发现中心，管理 Gateway/Send 实例及用户路由 |
| **Gateway** | 8084+ | 消息转发服务，维护 WebSocket 长连接，支持多实例部署 |
| **Send Service** | 8081 | 消息发送服务（无状态），负责消息转发与离线消息处理 |
| **Flusher** | - | 后台定时任务，将缓存中的脏数据（群组信息、消息）批量写入数据库 |

### 消息流转流程

```
Client ──WebSocket──▶ Gateway ◀──Redis Stream──┐
                                                 │（按 GatewayID 打包消息写入）
Client ──HTTP POST──▶ Send Service ─────────────┘
                         │
                         ├──▶ Meta Service（获取群成员信息）
                         └──▶ Registry Service（查询 UserID → GatewayID 路由）
```

1. 客户端通过 **Registry Service** 获取可用 Gateway 地址（负载均衡）
2. 客户端与 **Gateway** 建立 WebSocket 长连接
3. 发送消息时，客户端调用 **Send Service** HTTP API
4. Send Service 查询 **Meta Service** 获取群成员列表
5. Send Service 查询 **Registry Service** 获取每个用户所在的 Gateway （或本地缓存）
6. Send Service 将消息按 GatewayID 分组，写入对应的 **Redis Stream**
7. 各 Gateway 消费自己的 Redis Stream，通过 WebSocket 推送给客户端
8. 若用户不在线，消息存入 **Redis 离线消息队列**，用户上线后拉取

## 快速开始

### 前置依赖

- Go 1.25+
- MySQL 5.7+
- Redis 6.0+

### 1. 初始化数据库

```sql
CREATE DATABASE GoStacker CHARACTER SET utf8mb4;
USE GoStacker;

-- 执行 model/ 目录下的建表语句
SOURCE model/user.sql;
SOURCE model/chat_room.sql;
SOURCE model/chat_message.sql;
```

### 2. 修改配置

根据实际环境修改各 `config.*.yaml` 文件中的 MySQL、Redis 连接信息和 JWT 密钥。

### 3. 构建

```bash
# Windows
build.bat

# 或手动构建
go build -o bin/meta.exe ./cmd/meta
go build -o bin/registry.exe ./cmd/registry
go build -o bin/flusher.exe ./cmd/flusher
go build -o bin/send.exe ./cmd/send
go build -o bin/gateway.exe ./cmd/gateway
```

### 4. 启动服务

按以下顺序启动（有依赖关系）：

```bash
# Windows 一键启动
start.bat
```

手动启动顺序：
```bash
# 1. Meta Service（用户/群组元数据）
bin/meta.exe --config config.meta.yaml

# 2. Registry Service（注册发现中心）
bin/registry.exe --config config.registry.yaml

# 3. Flusher（缓存刷盘）
bin/flusher.exe --config config.flusher.yaml

# 4. Send Service（消息发送）
bin/send.exe --config config.send.yaml

# 5. Gateway（WebSocket 网关，可启动多个实例）
bin/gateway.exe --config config.gateway.yaml
bin/gateway2.exe --config config.gateway2.yaml
```

### 5. 测试客户端

项目附带 Python 客户端，支持注册、登录、发消息、WebSocket 接收：

```bash
cd client
pip install -r requirements.txt
python pyclient_gui.py
```


## 项目结构

```
GoStacker/
├── cmd/                          # 各微服务入口
│   ├── gateway/                  # Gateway 服务 (WebSocket 长连接 + 消息推送)
│   ├── send/                     # Send 服务 (消息发送与路由)
│   ├── meta/                     # Meta 服务 (用户与群组管理)
│   ├── registry/                 # Registry 服务 (服务注册与发现)
│   └── flusher/                  # Flusher 服务 (缓存刷盘)
│
├── internal/                     # 各服务业务逻辑 (私有)
│   ├── gateway/
│   │   ├── push/                 # 消息推送核心 (Dispatcher/Manager/Redis Stream)
│   │   ├── center/               # 内部转发接口
│   │   └── user/                 # WebSocket 连接处理
│   ├── send/
│   │   ├── chat/send/            # 消息发送 Handler/Service/Repo
│   │   ├── route/                # 用户路由查询 (UserID → GatewayID)
│   │   ├── pushback/             # Gateway 回调处理
│   │   └── pushnotify/           # 上线通知 & 离线消息推送
│   ├── meta/
│   │   ├── user/                 # 用户注册/登录
│   │   └── chat/group/           # 群组 CRUD、成员管理、加入审批
│   ├── registry/
│   │   ├── gateway/              # Gateway 实例注册/心跳/发现
│   │   ├── send/                 # Send 实例注册/心跳/发现
│   │   └── user/                 # 用户连接路由管理
│   └── server/                   # 通用服务器模块
│
├── pkg/                          # 公共基础包
│   ├── bootstrap/                # 统一初始化 (Config/Logger/MySQL/Redis/Monitor)
│   ├── config/                   # 配置定义与加载
│   ├── db/                       # MySQL & Redis 初始化封装
│   ├── logger/                   # Zap 日志 + Gin 中间件
│   ├── middleware/               # JWT 认证中间件
│   ├── monitor/                  # Prometheus 监控
│   ├── push/                     # 推送抽象层 (Standalone / Gateway 双模式)
│   ├── registry_client/          # 注册中心客户端 SDK
│   ├── response/                 # 统一响应格式
│   ├── pendingTask/              # 异步任务队列
│   └── utils/                    # 工具函数 (JWT/Snowflake 等)
│
├── model/                        # 数据库 SQL 建表语句
│   ├── user.sql                  # 用户表
│   ├── chat_room.sql             # 聊天室表
│   └── chat_message.sql          # 消息表
│
├── client/                       # 测试客户端 (Python CLI + GUI)
│   ├── pyclient.py               # 命令行客户端
│   └── pyclient_gui.py           # GUI 客户端
│
├── config.*.yaml                 # 各服务配置文件
├── build.bat                     # 一键构建脚本
├── start.bat                     # 一键启动脚本
└── go.mod
```



## 数据库设计

详细数据库设计文档请参考 [Database Design](docs/database.md)。

## API 概览

详细 API 接口文档请参考 [API Documentation](docs/api.md)。

## 路线图 (Roadmap)

- [x] **基础功能**: 1对1聊天、群聊、离线消息
- [x] **分布式架构**: 服务注册发现 (Registry)、网关负载均衡 (Gateway)
- [x] **可靠性**: 消息确认 (Ack)、写回缓存 (Write-Back)
- [ ] **性能优化**: 引入 Protobuf 序列化、连接池管理优化
- [ ] **可观测性**: 完善 Prometheus 监控指标
- [ ] **质量保障**: 添加自动化测试 (单元测试/集成测试)
- [ ] **部署**: 支持 Docker Compose 一键部署
- [ ] **云原生**: 支持 Kubernetes (Helm Charts)
- [ ] **功能增强**: 支持消息撤回、已读回执

## 参与贡献

欢迎提交 Issue 或 Pull Request！详情请查看 [CONTRIBUTING.md](CONTRIBUTING.md)。

## License

本项目仅用于学习交流。