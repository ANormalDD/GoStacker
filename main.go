package main

import (
	"GoStacker/cmd/server"
	"GoStacker/internal/chat/group"
	"GoStacker/pkg/config"
	"GoStacker/pkg/db/mysql"
	"GoStacker/pkg/db/redis"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/push"
	"GoStacker/pkg/utils"
	"fmt"
	"time"

	"go.uber.org/zap"
)

func main() {
	if err := config.Init(); err != nil {
		fmt.Printf("init config failed, err:%v\n", err)
		return
	}

	if err := logger.Init(config.Conf.LogConfig); err != nil {
		fmt.Printf("init logger failed, err:%v\n", err)
		return
	}
	defer zap.L().Sync()

	if err := mysql.Init(config.Conf.MySQLConfig); err != nil {
		zap.L().Fatal("init mysql failed", zap.Error(err))
		return
	}
	defer mysql.Close()
	if err := redis.Init(config.Conf.RedisConfig); err != nil {
		zap.L().Fatal("init redis failed", zap.Error(err))
		return
	}
	defer redis.Close()
	utils.SetJWTConfig(config.Conf.JWTConfig)
	push.InitDispatcher(config.Conf.DispatcherConfig)
	// start group flusher background worker (write-back cache) if enabled
	if config.Conf != nil && config.Conf.GroupCacheConfig != nil && config.Conf.GroupCacheConfig.Enabled {
		go func() {
			stopCh := make(chan struct{})
			go func() { /* never close stopCh; will run until process exit */ }()
			interval := 5 * time.Second
			if config.Conf.GroupCacheConfig.FlushIntervalSeconds > 0 {
				interval = time.Duration(config.Conf.GroupCacheConfig.FlushIntervalSeconds) * time.Second
			}
			batch := config.Conf.GroupCacheConfig.BatchSize
			if batch <= 0 {
				batch = 100
			}
			group.RunGroupFlusher(interval, batch, stopCh)
		}()
	}
	server.Start()

	defer zap.L().Info("service exit")
}
