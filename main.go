package main

import (
	"GoStacker/cmd/server"
	"GoStacker/pkg/config"
	"GoStacker/pkg/db/mysql"
	"GoStacker/pkg/db/redis"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/push"
	"GoStacker/pkg/utils"
	"fmt"

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
	server.Start()

	defer zap.L().Info("service exit")
}
