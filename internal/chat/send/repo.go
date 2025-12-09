package send

import (
	"GoStacker/pkg/db/mysql"
	"GoStacker/pkg/db/redis"
	"encoding/json"
	"strings"
	"time"

	snowflake "github.com/bwmarrin/snowflake"
	Redis "github.com/redis/go-redis/v9"
)

var sfNode *snowflake.Node

func init() {
	// 默认使用 node 1；在生产环境中可考虑从配置或机器 ID 派生节点号
	node, err := snowflake.NewNode(1)
	if err != nil {
		panic(err)
	}
	sfNode = node
}

func InsertMessage(roomID int64, senderID int64, content ChatPayload) (int64, error) {
	// 缓存写入：将消息序列化并推入 Redis 列表，后端定时批量写入 MySQL。
	contentData, err := json.Marshal(content)
	if err != nil {
		return 0, err
	}

	msgID := sfNode.Generate().Int64()

	cm := cachedMessage{
		ID:        msgID,
		RoomID:    roomID,
		SenderID:  senderID,
		Type:      content.GetType(),
		Content:   contentData,
		CreatedAt: time.Now(),
	}
	raw, err := json.Marshal(cm)
	if err != nil {
		return 0, err
	}

	// 使用 pkg/db/redis 的 RPushWithRetry 将缓存写入 Redis 列表：`cache:send:messages`
	// 注意：此处异步入队，立即返回生成的 msgID；最终会写入 MySQL
	if err := redis.RPushWithRetry(2, "cache:send:messages", raw); err != nil {
		return 0, err
	}
	return msgID, nil
}

// cachedMessage 是写入 Redis 的缓存结构
type cachedMessage struct {
	ID        int64           `json:"id"`
	RoomID    int64           `json:"room_id"`
	SenderID  int64           `json:"sender_id"`
	Type      string          `json:"type"`
	Content   json.RawMessage `json:"content"`
	CreatedAt time.Time       `json:"created_at"`
}

// StartMessageFlusher 启动一个后台循环，定期把 Redis 缓存队列中的消息批量写入 MySQL。
// - interval: 两次刷写的间隔
// - batchSize: 每次最大批量写入条数
// - stopCh: 若传入非 nil 的 channel，关闭该 channel 可停止刷写循环
func StartMessageFlusher(interval time.Duration, batchSize int, stopCh chan struct{}) {
	if batchSize <= 0 {
		batchSize = 100
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				flushOnce(batchSize)
			case <-stopCh:
				return
			}
		}
	}()
}

// flushOnce 从 Redis 中弹出最多 batchSize 条缓存并批量写入 MySQL。
func flushOnce(batchSize int) {
	var msgs []cachedMessage
	for i := 0; i < batchSize; i++ {
		s, err := redis.LPopWithRetry(2, "cache:send:messages")
		if err != nil {
			if err == Redis.Nil {
				break
			}
			// 如果 LPop 出错，记录并中断本轮（避免无限错误循环）
			// log via mysql package's logger or global logger if available
			break
		}
		var cm cachedMessage
		if err := json.Unmarshal([]byte(s), &cm); err != nil {
			// 无法解析则跳过
			continue
		}
		msgs = append(msgs, cm)
	}
	if len(msgs) == 0 {
		return
	}

	if err := insertBatch(msgs); err != nil {
		for i := len(msgs) - 1; i >= 0; i-- {
			raw, _ := json.Marshal(msgs[i])
			_ = redis.RPushWithRetry(2, "cache:send:messages", raw)
		}
	}
}

func insertBatch(msgs []cachedMessage) error {

	query := "INSERT INTO chat_messages (id, room_id, sender_id, type, content, created_at) VALUES "
	vals := make([]interface{}, 0, len(msgs)*6)
	placeholders := make([]string, 0, len(msgs))
	for _, m := range msgs {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?)")
		vals = append(vals, m.ID, m.RoomID, m.SenderID, m.Type, []byte(m.Content), m.CreatedAt)
	}
	query += strings.Join(placeholders, ",")

	_, err := mysql.DB.Exec(query, vals...)
	return err
}
