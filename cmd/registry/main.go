package main

import (
	"GoStacker/pkg/bootstrap"
	"GoStacker/pkg/config"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

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

	// Start background cleanup goroutine
	go startBackgroundCleanup()

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

// startBackgroundCleanup starts a goroutine that periodically cleans up expired entries
func startBackgroundCleanup() {
	// Default cleanup interval: 10 seconds
	interval := 10 * time.Second
	if config.Conf != nil && config.Conf.RegistryConfig != nil {
		if config.Conf.RegistryConfig.CleanupInterval > 0 {
			interval = time.Duration(config.Conf.RegistryConfig.CleanupInterval) * time.Second
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	zap.L().Info("Background cleanup started", zap.Duration("interval", interval))

	for range ticker.C {
		// Cleanup logic will be handled by Redis TTL automatically
		// This is just a placeholder for any additional cleanup tasks
		// For example, removing from sorted sets when gateway/send is expired
		zap.L().Debug("Background cleanup tick")
	}
}
