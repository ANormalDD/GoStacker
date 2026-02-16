# API 概览

## Meta Service (用户与群组)

| Method | Path | 说明 | 认证 |
|--------|------|------|------|
| POST | `/register` | 用户注册 | ✗ |
| POST | `/login` | 用户登录（返回 JWT） | ✗ |
| POST | `/api/chat/group/create` | 创建群组 | ✓ |
| POST | `/api/chat/group/add_member` | 添加群成员 | ✓ |
| POST | `/api/chat/group/add_members` | 批量添加群成员 | ✓ |
| POST | `/api/chat/group/remove_member` | 移除群成员 | ✓ |
| POST | `/api/chat/group/join` | 加入群组 | ✓ |
| POST | `/api/chat/group/join/request` | 申请加入群组 | ✓ |
| GET | `/api/chat/group/join/requests` | 查看待审批申请 | ✓ |
| POST | `/api/chat/group/join/respond` | 审批加入申请 | ✓ |
| POST | `/api/chat/group/change_nickname` | 修改群昵称 | ✓ |
| POST | `/api/chat/group/change_member_role` | 修改成员角色 | ✓ |
| GET | `/api/chat/group/search` | 搜索群组 | ✓ |
| GET | `/api/joined_rooms` | 获取已加入的群组 | ✓ |

## Send Service (消息发送)

| Method | Path | 说明 | 认证 |
|--------|------|------|------|
| POST | `/api/chat/send_message` | 发送聊天消息 | ✓ |
| POST | `/api/chat/resend_message` | 重发消息 | ✓ |
| POST | `/internal/pushback` | Gateway 回调（内部） | ✗ |
| POST | `/internal/push/notify_online` | 上线通知（内部） | ✗ |

## Gateway (WebSocket 连接)

| Method | Path | 说明 | 认证 |
|--------|------|------|------|
| GET | `/api/ws` | WebSocket 长连接 | ✓ |
| POST | `/center/forward` | 内部消息转发 | ✗ |

## Registry Service (服务发现)

| Method | Path | 说明 |
|--------|------|------|
| POST | `/registry/gateway/register` | 注册 Gateway |
| POST | `/registry/gateway/heartbeat` | Gateway 心跳 |
| DELETE | `/registry/gateway/:gateway_id` | 注销 Gateway |
| GET | `/registry/gateway/instances` | 列出 Gateway 实例 |
| GET | `/registry/gateway/available` | 获取可用 Gateway（负载均衡） |
| POST | `/registry/send/register` | 注册 Send 实例 |
| POST | `/registry/send/heartbeat` | Send 心跳 |
| DELETE | `/registry/send/:instance_id` | 注销 Send 实例 |
| GET | `/registry/send/instances` | 列出 Send 实例 |
| GET | `/registry/send/available` | 获取可用 Send 实例 |
| POST | `/registry/user/connect` | 上报用户连接 |
| POST | `/registry/user/disconnect` | 上报用户断连 |
| POST | `/registry/user/routes/batch` | 批量查询用户路由 |
