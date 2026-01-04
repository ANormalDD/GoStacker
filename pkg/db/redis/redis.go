package redis

import (
	"GoStacker/pkg/config"
	"GoStacker/pkg/monitor"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var Rdb *redis.Client
var Monitor *monitor.Monitor

func Init(cfg *config.RedisConfig) (err error) {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password, // no password set
		DB:       cfg.DB,       // use default DB
		PoolSize: cfg.PoolSize,
	})
	Monitor = monitor.NewMonitor("redis", 1000, 10000, 60000)
	// register hook so all redis commands are monitored centrally
	Rdb.AddHook(&redisMonitorHook{mon: Monitor})
	ctx := context.Background()
	_, err = Rdb.Ping(ctx).Result()
	return
}

func LPopWithRetry(retry int, key string) (string, error) {
	var err error
	var result string
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		result, err = Rdb.LPop(ctx, key).Result()
		if err == nil {
			return result, nil
		}
		if err == redis.Nil {
			return "", redis.Nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}
func RPushWithRetry(retry int, key string, value interface{}) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		err = Rdb.RPush(ctx, key, value).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// set operation
func SAddWithRetry(retry int, key string, members ...interface{}) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		err = Rdb.SAdd(ctx, key, members...).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// query everything in set
func SMembersWithRetry(retry int, key string) ([]string, error) {
	var err error
	var result []string
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		result, err = Rdb.SMembers(ctx, key).Result()
		if err == nil {
			return result, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}
func SScanWithRetry(retry int, key string, cursor uint64, match string, count int64) ([]string, uint64, error) {
	var err error
	var result []string
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		result, cursor, err = Rdb.SScan(ctx, key, cursor, match, count).Result()
		if err == nil {
			return result, cursor, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, cursor, err
}
func SIsEmptyWithRetry(retry int, key string) (bool, error) {
	var err error
	var count int64
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		count, err = Rdb.SCard(ctx, key).Result()
		if err == nil {
			return count == 0, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false, err
}
func XAddWithRetry(retry int, stream string, values map[string]interface{}) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		_, err = Rdb.XAdd(ctx, &redis.XAddArgs{
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
func XGroupCreateMkStreamWithRetry(retry int, stream string, group string, start string) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		err = Rdb.XGroupCreateMkStream(ctx, stream, group, start).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

func XReadGroupBlocking(stream string, group string, consumer string, count int64, block time.Duration, lastID string) ([]redis.XStream, error) {
	ctx := context.Background()
	return Rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, lastID},
		Count:    count,
		Block:    block,
	}).Result()
}

func XAckWithRetry(retry int, stream string, group string, ids ...string) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		err = Rdb.XAck(ctx, stream, group, ids...).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// SetEXWithRetry sets a key with an expiration time
func SetEXWithRetry(retry int, key string, value interface{}, expiration time.Duration) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		err = Rdb.SetEx(ctx, key, value, expiration).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// GetWithRetry gets a value by key
func GetWithRetry(retry int, key string) (string, error) {
	var err error
	var result string
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		result, err = Rdb.Get(ctx, key).Result()
		if err == nil {
			return result, nil
		}
		if err == redis.Nil {
			return "", redis.Nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}

// MGetWithRetry gets multiple values by keys
func MGetWithRetry(retry int, keys []string) ([]string, error) {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		result, err := Rdb.MGet(ctx, keys...).Result()
		if err == nil {
			// Convert []interface{} to []string
			strResult := make([]string, len(result))
			for i, v := range result {
				if v == nil {
					strResult[i] = ""
				} else {
					strResult[i] = v.(string)
				}
			}
			return strResult, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, err
}

// DelWithRetry deletes a key
func DelWithRetry(retry int, key string) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		err = Rdb.Del(ctx, key).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// ExistsWithRetry checks if a key exists
func ExistsWithRetry(retry int, key string) (int64, error) {
	var err error
	var result int64
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		result, err = Rdb.Exists(ctx, key).Result()
		if err == nil {
			return result, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}

// ExpireWithRetry sets expiration on a key
func ExpireWithRetry(retry int, key string, expiration time.Duration) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		err = Rdb.Expire(ctx, key, expiration).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// ZAddWithRetry adds member to sorted set
func ZAddWithRetry(retry int, key string, score float64, member string) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		err = Rdb.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// ZRangeWithScoresWithRetry gets members from sorted set with scores
func ZRangeWithScoresWithRetry(retry int, key string, start, stop int64) ([]string, error) {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		result, err := Rdb.ZRange(ctx, key, start, stop).Result()
		if err == nil {
			return result, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, err
}

// ZRemWithRetry removes member from sorted set
func ZRemWithRetry(retry int, key string, members ...string) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		err = Rdb.ZRem(ctx, key, members).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// SRemWithRetry removes member from set
func SRemWithRetry(retry int, key string, members ...interface{}) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		err = Rdb.SRem(ctx, key, members...).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// SCardWithRetry gets set cardinality
func SCardWithRetry(retry int, key string) (int64, error) {
	var err error
	var result int64
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		result, err = Rdb.SCard(ctx, key).Result()
		if err == nil {
			return result, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}

// SRandMemberWithRetry gets a random member from set
func SRandMemberWithRetry(retry int, key string) (string, error) {
	var err error
	var result string
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		result, err = Rdb.SRandMember(ctx, key).Result()
		if err == nil {
			return result, nil
		}
		if err == redis.Nil {
			return "", redis.Nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}

// Ping checks Redis connection
func Ping() (string, error) {
	ctx := context.Background()
	return Rdb.Ping(ctx).Result()
}

func Close() {
	_ = Rdb.Close()
}
