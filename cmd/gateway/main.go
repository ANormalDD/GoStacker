package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"GoStacker/internal/gateway/center_client"
	cws "GoStacker/internal/gateway/center_client/ws"
	"GoStacker/internal/gateway/push"
	"GoStacker/pkg/bootstrap"
	"GoStacker/pkg/config"

	"go.uber.org/zap"
)

func main() {
	cfgPath := flag.String("config", "config.gateway.yaml", "path to gateway config yaml")
	flag.Parse()

	cleanup, err := bootstrap.InitAll(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init bootstrap: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	// init push dispatcher for gateway
	if config.Conf != nil && config.Conf.GatewayDispatcherConfig != nil {
		push.InitDispatcher(config.Conf.GatewayDispatcherConfig)
	}

	// register to center in background
	gatewayAddr := config.Conf.Address
	gatewayPort := config.Conf.Port
	maxConns := 0
	if config.Conf.GatewayDispatcherConfig != nil {
		maxConns = config.Conf.GatewayDispatcherConfig.MaxConnections
	}
	go func() {
		if config.Conf.CenterConfig == nil {
			zap.L().Warn("center config nil, skipping register to center")
			return
		}
		if err := center_client.RegisterToCenter(config.Conf.CenterConfig, gatewayAddr, gatewayPort, maxConns); err != nil {
			zap.L().Warn("register to center failed", zap.Error(err))
		} else {
			zap.L().Info("registered to center successfully")
		}
	}()

	// start HTTP server
	r := InitRouter()
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Conf.Port),
		Handler: r,
	}

	go func() {
		zap.L().Info("starting gateway http server", zap.Int("port", config.Conf.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("http server error", zap.Error(err))
		}
	}()

	// wait for termination
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zap.L().Info("shutting down gateway server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		zap.L().Error("server shutdown error", zap.Error(err))
	}

	// stop center ws
	if err := cws.Stop(); err != nil {
		zap.L().Warn("failed to stop center ws", zap.Error(err))
	}

	zap.L().Info("gateway server exited")
}
