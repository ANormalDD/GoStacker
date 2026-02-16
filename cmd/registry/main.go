package main

import (
	"GoStacker/pkg/bootstrap"
	"GoStacker/pkg/config"
	"flag"
	"fmt"
	"net/http"
	"os"

	"go.uber.org/zap"
)

func main() {
	cfgPath := flag.String("config", "config.registry.yaml", "path to config yaml")
	flag.Parse()

	cleanup, err := bootstrap.InitAll(*cfgPath)
	if err != nil {
		fmt.Printf("bootstrap init failed: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	// Build registry service router and start server
	engine := NewRouter()
	addr := fmt.Sprintf(":%d", config.Conf.Port)
	serverSrv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	zap.L().Info("Registry service starting", zap.String("address", addr))
	fmt.Printf("Registry service listening on %s\n", addr)

	if err := serverSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		zap.L().Fatal("Registry service listen failed", zap.Error(err))
		fmt.Printf("Registry service listen failed: %v\n", err)
		os.Exit(1)
	}
}

