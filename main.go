package main

import (
	"GoStacker/cmd/server"
	"GoStacker/pkg/config"
	"GoStacker/pkg/db/mysql"
	"GoStacker/pkg/logger"
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
	utils.SetJWTConfig(config.Conf.JWTConfig)
	server.Start()

	defer zap.L().Info("service exit")
}
