package redis

import (
	"GoStacker/pkg/config"
	"GoStacker/pkg/monitor"
	"context"
	"fmt"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	sendRedisRoleStream = "stream"
	sendRedisRoleQueue  = "queue"
	sendRedisRoleCache  = "cache"
)

var (
	sendRoleClientsMu sync.RWMutex
	sendRoleClients   = make(map[string]*goredis.Client)
)

// InitSendRoleClients initializes send role-based redis clients.
// Roles not configured in send_redis fall back to the default global redis client.
func InitSendRoleClients(defaultCfg *config.RedisConfig, sendCfg *config.SendRedisConfig) error {
	sendRoleClientsMu.Lock()
	defer sendRoleClientsMu.Unlock()

	closeSendRoleClientsLocked()

	baseClient := Rdb
	if baseClient == nil {
		if defaultCfg == nil {
			return fmt.Errorf("default redis config is nil")
		}

		client, err := newRedisClient(defaultCfg, "redis")
		if err != nil {
			return fmt.Errorf("init default redis client failed: %w", err)
		}
		Rdb = client
		baseClient = client
	}

	roleCfgs := map[string]*config.RedisConfig{
		sendRedisRoleStream: nil,
		sendRedisRoleQueue:  nil,
		sendRedisRoleCache:  nil,
	}
	if sendCfg != nil {
		roleCfgs[sendRedisRoleStream] = sendCfg.Stream
		roleCfgs[sendRedisRoleQueue] = sendCfg.Queue
		roleCfgs[sendRedisRoleCache] = sendCfg.Cache
	}

	for role, cfg := range roleCfgs {
		if cfg == nil {
			sendRoleClients[role] = baseClient
			continue
		}

		client, err := newRedisClient(cfg, "redis_send_"+role)
		if err != nil {
			closeSendRoleClientsLocked()
			return fmt.Errorf("init send redis role %s failed: %w", role, err)
		}
		sendRoleClients[role] = client
	}

	for role, client := range sendRoleClients {
		if client == nil {
			continue
		}
		opt := client.Options()
		zap.L().Info("Send redis role initialized",
			zap.String("role", role),
			zap.String("addr", opt.Addr),
			zap.Int("db", opt.DB),
			zap.Bool("dedicated", client != baseClient))
	}
	return nil
}

func newRedisClient(cfg *config.RedisConfig, monitorName string) (*goredis.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redis config is nil")
	}
	client := goredis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	mon := monitor.NewMonitor(monitorName, 1000, 10000, 60000)
	client.AddHook(&redisMonitorHook{mon: mon})
	if _, err := client.Ping(context.Background()).Result(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func getSendRoleClient(role string) *goredis.Client {
	sendRoleClientsMu.RLock()
	client := sendRoleClients[role]
	sendRoleClientsMu.RUnlock()
	if client != nil {
		return client
	}
	return Rdb
}

func closeSendRoleClientsLocked() {
	seen := make(map[*goredis.Client]struct{})
	for _, client := range sendRoleClients {
		if client == nil || client == Rdb {
			continue
		}
		if _, ok := seen[client]; ok {
			continue
		}
		_ = client.Close()
		seen[client] = struct{}{}
	}
	sendRoleClients = make(map[string]*goredis.Client)
}

func CloseSendRoleClients() {
	sendRoleClientsMu.Lock()
	defer sendRoleClientsMu.Unlock()
	closeSendRoleClientsLocked()
}

func sendRPushWithRetry(client *goredis.Client, retry int, key string, value interface{}) error {
	if client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	var err error
	for i := 0; i < retry; i++ {
		err = client.RPush(context.Background(), key, value).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

func sendLPopWithRetry(client *goredis.Client, retry int, key string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("redis client not initialized")
	}
	var (
		err    error
		result string
	)
	for i := 0; i < retry; i++ {
		result, err = client.LPop(context.Background(), key).Result()
		if err == nil {
			return result, nil
		}
		if err == goredis.Nil {
			return "", goredis.Nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}

func sendLLenWithRetry(client *goredis.Client, retry int, key string) (int64, error) {
	if client == nil {
		return 0, fmt.Errorf("redis client not initialized")
	}
	var (
		err    error
		result int64
	)
	for i := 0; i < retry; i++ {
		result, err = client.LLen(context.Background(), key).Result()
		if err == nil {
			return result, nil
		}
		if err == goredis.Nil {
			return 0, goredis.Nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}

func sendXAddWithRetry(client *goredis.Client, retry int, stream string, values map[string]interface{}) error {
	if client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	var err error
	for i := 0; i < retry; i++ {
		_, err = client.XAdd(context.Background(), &goredis.XAddArgs{
			Stream: stream,
			Values: values,
		}).Result()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

func SendStreamXAddWithRetry(retry int, stream string, values map[string]interface{}) error {
	return sendXAddWithRetry(getSendRoleClient(sendRedisRoleStream), retry, stream, values)
}

func SendQueueRPushWithRetry(retry int, key string, value interface{}) error {
	return sendRPushWithRetry(getSendRoleClient(sendRedisRoleQueue), retry, key, value)
}

func SendQueueLPopWithRetry(retry int, key string) (string, error) {
	return sendLPopWithRetry(getSendRoleClient(sendRedisRoleQueue), retry, key)
}

func SendQueueLLenWithRetry(retry int, key string) (int64, error) {
	return sendLLenWithRetry(getSendRoleClient(sendRedisRoleQueue), retry, key)
}

func SendCacheRPushWithRetry(retry int, key string, value interface{}) error {
	return sendRPushWithRetry(getSendRoleClient(sendRedisRoleCache), retry, key, value)
}

func SendCacheLPopWithRetry(retry int, key string) (string, error) {
	return sendLPopWithRetry(getSendRoleClient(sendRedisRoleCache), retry, key)
}
