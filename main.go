package main

import (
	"GoStacker/cmd/server"
	"GoStacker/internal/meta/chat/group"
	chatsend "GoStacker/internal/send/chat/send"
	"GoStacker/pkg/config"
	"GoStacker/pkg/db/mysql"
	"GoStacker/pkg/db/redis"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/monitor"
	"GoStacker/pkg/push"
	"GoStacker/pkg/utils"
	"fmt"
	"runtime"
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
	monitor.InitMonitor()
	if config.Conf.PushMod == "standalone" {
		push.InitDispatcher(config.Conf.SendDispatcherConfig)
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
		// start message flusher for cached send->mysql writes (default interval 5s, batch 100)
		go func() {
			stopCh := make(chan struct{})
			go func() { /* never close stopCh; will run until process exit */ }()
			interval := 5 * time.Second
			batch := 100
			chatsend.StartMessageFlusher(interval, batch, stopCh)
		}()
	} else {
		// Gateway mode: start gateway dispatcher worker pool
		gwWorkers := 0
		gwQueue := 0
		if config.Conf != nil && config.Conf.SendDispatcherConfig != nil {
			gwWorkers = config.Conf.SendDispatcherConfig.GatewayWorkerCount
			gwQueue = config.Conf.SendDispatcherConfig.GatewayQueueSize
		}
		if gwWorkers <= 0 {
			gwWorkers = runtime.NumCPU()
		}
		if gwQueue <= 0 {
			gwQueue = 1024
		}
		push.StartGatewayDispatcher(gwWorkers, gwQueue)
	}
	server.Start(config.Conf.PushMod)
	defer zap.L().Info("service exit")
}
