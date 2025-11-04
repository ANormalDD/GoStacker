package server

import (
	"GoStacker/internal/server"
	"GoStacker/pkg/config"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

func Start(PushMod string) {

	engine := server.NewRouter(PushMod)

	// Print registered routes for visibility
	for _, ri := range engine.Routes() {
		// ri is gin.RouteInfo
		zap.L().Info("route", zap.String("method", ri.Method), zap.String("path", ri.Path), zap.String("handler", ri.Handler))
	}

	addr := fmt.Sprintf(":%d", config.Conf.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}
	zap.L().Info("server run", zap.String("addr", addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		zap.L().Fatal("listen: %s\n", zap.Error(err))
	}
}
