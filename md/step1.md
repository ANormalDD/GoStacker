# 目标
实现高性能聊天室功能，支持群聊，权限管理

# 建表

## users 表
```sql
CREATE TABLE users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT '用户ID',
    username VARCHAR(50) NOT NULL UNIQUE COMMENT '用户名',
    password_hash VARCHAR(255) NOT NULL COMMENT '加密后的密码',
    nickname VARCHAR(50) DEFAULT NULL COMMENT '用户昵称',
    avatar_url VARCHAR(255) DEFAULT NULL COMMENT '头像链接',
    is_online BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否在线',
    is_banned BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否封禁',
    last_login_at DATETIME DEFAULT NULL COMMENT '最后登录时间',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '注册时间',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
```

## chat_rooms 表（聊天室）
```sql
CREATE TABLE chat_rooms (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT '聊天室ID',
    name VARCHAR(100) NOT NULL COMMENT '聊天室名称',
    is_group BOOLEAN NOT NULL DEFAULT TRUE COMMENT '是否为群聊',
    creator_id BIGINT UNSIGNED NOT NULL COMMENT '创建者ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    FOREIGN KEY (creator_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='聊天室表';
```

## chat_room_members表 (按照roomid分表)
考虑到多用户以及多群聊的问题所带来的性能问题，这里我们使用roomid来分表
