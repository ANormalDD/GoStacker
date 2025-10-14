package main

import (
	"GoStacker/internal/server"
	"GoStacker/pkg/config"
	"GoStacker/pkg/db/mysql"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/utils"
	"fmt"
	"net/http"

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
	r := server.NewRouter()
	addr := fmt.Sprintf(":%d", config.Conf.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}
	zap.L().Info("server run", zap.String("addr", addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		zap.L().Fatal("listen: %s\n", zap.Error(err))
	}
}
