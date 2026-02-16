# 数据库设计

## 用户表 (`users`)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT UNSIGNED | 用户 ID（主键自增） |
| username | VARCHAR(50) | 用户名（唯一） |
| password_hash | VARCHAR(255) | 加密密码 (bcrypt) |
| nickname | VARCHAR(50) | 昵称 |
| avatar_url | VARCHAR(255) | 头像链接 |
| joined_chatrooms | TEXT | 已加入的聊天室 ID 列表 |
| is_online | BOOLEAN | 是否在线 |
| is_banned | BOOLEAN | 是否封禁 |

## 聊天室表 (`chat_rooms`)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT UNSIGNED | 聊天室 ID |
| name | VARCHAR(100) | 聊天室名称 |
| is_group | BOOLEAN | 是否为群聊 |
| creator_id | BIGINT UNSIGNED | 创建者 ID |

## 消息表 (`chat_messages`)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT | 消息 ID (Snowflake) |
| room_id | BIGINT | 聊天室 ID |
| sender_id | BIGINT | 发送者 ID |
| content | TEXT | 消息内容 |
| type | VARCHAR(20) | 消息类型 (text/image/file/system) |
| is_deleted | BOOLEAN | 是否已删除 |
