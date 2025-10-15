package redis

import (
	"fmt"
	"GoStacker/pkg/config"

	"github.com/go-redis/redis"
)

var Rdb *redis.Client

func Init(cfg *config.RedisConfig) (err error) {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password, // no password set
		DB:       cfg.DB,       // use default DB
		PoolSize: cfg.PoolSize,
	})
	_, err = Rdb.Ping().Result()
	return
}
func Close() {
	_ = Rdb.Close()
}
