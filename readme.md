# GoStacker

![Go Version](https://img.shields.io/github/go-mod/go-version/ANormalDD/GoStacker)
[![Go Report Card](https://goreportcard.com/badge/github.com/ANormalDD/GoStacker)](https://goreportcard.com/report/github.com/ANormalDD/GoStacker)
![License](https://img.shields.io/github/license/ANormalDD/GoStacker)

> **GoStacker** is a high-performance, scalable distributed Instant Messaging (IM) system.
> It adopts a microservices architecture, supporting **multi-gateway load balancing**, **reliable message delivery**, and **write-diffusion optimization**. It is designed to help developers deeply understand the architectural design and implementation details of distributed IM systems.

[中文文档](README_zh-CN.md)

## Tech Stack

- **Language**: Go 1.25+
- **Web Framework**: [Gin](https://github.com/gin-gonic/gin)
- **WebSocket**: [gorilla/websocket](https://github.com/gorilla/websocket)
- **Database**: MySQL (via [sqlx](https://github.com/jmoiron/sqlx) + [sqlhooks](https://github.com/qustavo/sqlhooks))
- **Cache/MQ**: Redis (go-redis v9), using Redis Stream as Message Queue
- **Auth**: JWT ([golang-jwt/jwt](https://github.com/golang-jwt/jwt))
- **ID Generator**: [Snowflake](https://github.com/bwmarrin/snowflake) Distributed ID
- **Config**: [Viper](https://github.com/spf13/viper) + YAML
- **Logging**: [Zap](https://go.uber.org/zap) + [Lumberjack](https://github.com/natefinch/lumberjack) for rotation
- **Monitoring**: [Prometheus](https://github.com/prometheus/client_golang) metrics (`/metrics`)
- **Encryption**: golang.org/x/crypto (bcrypt)
- **Local Cache**: [Ristretto](https://github.com/dgraph-io/ristretto)

## Key Features

### 1. Microservices Architecture
Split into four independent services: Gateway (Connection Management), Send (Message Routing), Meta (User/Group Management), and Registry (Service Discovery). Each service is deployed independently with clear responsibilities.

### 2. Self-Adaptive Service Discovery & Load Balancing
Implements real-time load ranking of Gateway instances using Redis ZSet. Combines heartbeat health checks with TTL auto-expiration to achieve automatic failure detection and least-load routing.

### 3. High-Concurrency WebSocket Management
Uses `sync.Map` for connection mapping. Each connection has an independent send queue and Writer goroutine to avoid concurrent write conflicts. Supports automatic draining and redelivery of unsent messages during connection migration.

### 4. Write-Back Caching
Group member data is written to Redis cache and marked as dirty (ZSet timestamp sorting). The background Flusher service periodically batches dirty data to MySQL. Uses Redis Pipeline to atomically clear dirty flags and set short TTLs, balancing read performance and data consistency.

### 5. Reliable Message Delivery
Implements a Sharded Lock Pending Task Manager (64 shards + atomic) to track the delivery status of every message. Automatically triggers a PushBack mechanism to route messages back to the Send Service when a user is offline.

### 6. Observability
Self-developed Sliding Window Monitor (Ring Buffer + Async Insertion) stats API latency and success rates in real-time. Integrates with Prometheus to expose `/metrics`. Uses structured logging with Zap and log rotation with Lumberjack.

### 7. User Routing Lease Optimization
When a user disconnects, the User→Gateway route is not deleted immediately but marked as `disconnected` with a retained TTL (configurable). If the user reconnects shortly, the original Gateway is reused to avoid load balancing jitter and connection migration overheads. Registry and Send services support local caching of User→Gateway routes (TTL shorter than Redis) to reduce high-frequency network round-trips.

## Architecture Overview

![architecture](structure.png)

The system consists of **5 Microservices**:

| Service | Default Port | Description |
|---------|--------------|-------------|
| **Meta Service** | 8082 | User/Group metadata management (Register, Login, Group CRUD). |
| **Registry Service** | 8083 | Service discovery center. Manages Gateway/Send instances and User routing. |
| **Gateway** | 8084+ | Message forwarding service. Maintains WebSocket persistent connections. Supports multi-instance deployment. |
| **Send Service** | 8081 | Stateless message sending service. Handles message routing and offline storage. |
| **Flusher** | - | Background scheduled task. Batches dirty data (Group info, Messages) from cache to DB. |

### Message Flow

```
Client ──WebSocket──▶ Gateway ◀──Redis Stream──┐
                                                 │ (Grouped by GatewayID)
Client ──HTTP POST──▶ Send Service ─────────────┘
                         │
                         ├──▶ Meta Service (Get Group Members)
                         └──▶ Registry Service (Query UserID → GatewayID Route)
```

1. Client gets available Gateway address from **Registry Service** (Load Balancing).
2. Client establishes WebSocket connection with **Gateway**.
3. To send a message, Client calls **Send Service** HTTP API.
4. Send Service queries **Meta Service** for group members.
5. Send Service queries **Registry Service** for the Gateway of each user (or uses local cache).
6. Send Service groups messages by GatewayID and writes to corresponding **Redis Stream**.
7. Each Gateway consumes its own Redis Stream and pushes to clients via WebSocket.
8. If user is offline, message is stored in **Redis Offline Queue** for fetch upon reconnection.

## Quick Start

### Prerequisites

- Go 1.25+
- MySQL 5.7+
- Redis 6.0+

### 1. Initialize Database

```sql
CREATE DATABASE GoStacker CHARACTER SET utf8mb4;
USE GoStacker;

-- Execute SQL files in model/ directory
SOURCE model/user.sql;
SOURCE model/chat_room.sql;
SOURCE model/chat_message.sql;
```

### 2. Configure

Modify `config.*.yaml` files to match your MySQL, Redis connection info and JWT secret.

### 3. Build

```bash
# Windows
build.bat

# Or manual build
go build -o bin/meta.exe ./cmd/meta
go build -o bin/registry.exe ./cmd/registry
go build -o bin/flusher.exe ./cmd/flusher
go build -o bin/send.exe ./cmd/send
go build -o bin/gateway.exe ./cmd/gateway
```

### 4. Run

Start in the following order (due to dependencies):

```bash
# Windows One-Click Start
start.bat
```

Manual Start Order:
```bash
# 1. Meta Service (User/Group Metadata)
bin/meta.exe --config config.meta.yaml

# 2. Registry Service (Service Discovery)
bin/registry.exe --config config.registry.yaml

# 3. Flusher (Cache Persistence)
bin/flusher.exe --config config.flusher.yaml

# 4. Send Service (Message Sending)
bin/send.exe --config config.send.yaml

# 5. Gateway (WebSocket Gateway, multiple instances supported)
bin/gateway.exe --config config.gateway.yaml
bin/gateway2.exe --config config.gateway2.yaml
```

### 5. Test Client

A Python client is included for testing registration, login, sending messages, and WebSocket receiving:

```bash
cd client
pip install -r requirements.txt

python pyclient_gui.py
```

## Project Structure

```
GoStacker/
├── cmd/                          # Entry points for microservices
│   ├── gateway/                  # Gateway Service (WebSocket + Push)
│   ├── send/                     # Send Service (Routing + Offline Msg)
│   ├── meta/                     # Meta Service (User & Group Mgmt)
│   ├── registry/                 # Registry Service (Discovery)
│   └── flusher/                  # Flusher Service (Cache Persistence)
│
├── internal/                     # Private business logic
│   ├── gateway/                  # Gateway logic
│   ├── send/                     # Send logic
│   ├── meta/                     # Meta logic
│   ├── registry/                 # Registry logic
│   └── server/                   # Common server module
│
├── pkg/                          # Public shared packages
│   ├── bootstrap/                # Initialization (Config/Logger/DB)
│   ├── config/                   # Configuration loading
│   ├── db/                       # Database wrappers
│   ├── logger/                   # Zap logger
│   ├── middleware/               # HTTP middlewares (JWT)
│   ├── monitor/                  # Prometheus monitoring
│   ├── push/                     # Push abstraction
│   ├── registry_client/          # Registry client SDK
│   └── utils/                    # Utilities (Snowflake, etc.)
│
├── model/                        # SQL schemas
├── client/                       # Python Test Client
├── config.*.yaml                 # Configuration files
├── build.bat                     # Build script
└── start.bat                     # Start script
```

## Database Design

See [Database Design](docs/database.md) for details.

## API Documentation

See [API Documentation](docs/api.md) for details.

## Roadmap

- [x] **Basic Features**: 1-on-1 Chat, Group Chat, Offline Messaging
- [x] **Distributed Arch**: Service Discovery (Registry), Gateway Load Balancing
- [x] **Reliability**: Message ACK, Write-Back Caching
- [ ] **Performance**: Protobuf support, Connection Pool Optimization
- [ ] **Observability**: Enhance Prometheus Metrics
- [ ] **Quality Assurance**: Automated Testing (Unit/Integration)
- [ ] **Deployment**: Docker Compose support
- [ ] **Cloud Native**: Kubernetes (Helm Charts) support
- [ ] **Enhancement**: Message Recall, Read Receipts

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

This project is for learning and research purposes.
