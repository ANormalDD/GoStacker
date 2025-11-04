package server

import (
	"GoStacker/internal/server"
	"GoStacker/pkg/config"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

func Start(PushMod string) {

	r := server.NewRouter(PushMod)
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
