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

	"GoStacker/internal/gateway/push"
	"GoStacker/pkg/bootstrap"
	"GoStacker/pkg/config"
	"GoStacker/pkg/registry_client"

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

	// Initialize Registry client and register gateway
	var registryClient *registry_client.GatewayClient
	var heartbeatStopCh chan struct{}
	if config.Conf != nil && config.Conf.RegistryConfig.URL != "" {
		gatewayID := fmt.Sprintf("gateway-%s-%d", config.Conf.Name, config.Conf.MachineID)
		registryClient = registry_client.NewGatewayClient(config.Conf.RegistryConfig.URL, gatewayID)

		// Set registry client for push module to report user connections
		push.SetRegistryClient(registryClient)

		// Register with Registry service
		maxConns := 0
		if config.Conf.GatewayDispatcherConfig != nil {
			maxConns = config.Conf.GatewayDispatcherConfig.MaxConnections
		}
		err = registryClient.Register(config.Conf.Address, config.Conf.Port, 10000)
		if err != nil {
			zap.L().Error("Failed to register with registry service", zap.Error(err))
			fmt.Printf("Failed to register with registry: %v\n", err)
			// Continue anyway, will retry on heartbeat
		} else {
			zap.L().Info("Gateway registered with registry",
				zap.String("gateway_id", gatewayID),
				zap.String("address", config.Conf.Address),
				zap.Int("port", config.Conf.Port))
		}

		// Start heartbeat to report load
		heartbeatStopCh = make(chan struct{})
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					// Calculate current load
					connCount := push.GetConnectionCount()
					load := float32(0.0)
					if maxConns > 0 {
						load = float32(connCount) / float32(maxConns)
					}
					// Send heartbeat
					if err := registryClient.Heartbeat(load, connCount, 0.0, 0); err != nil {
						zap.L().Warn("Failed to send heartbeat to registry", zap.Error(err))
					}
				case <-heartbeatStopCh:
					return
				}
			}
		}()

		// Initialize Redis Stream consumption
		if config.Conf.GatewayDispatcherConfig != nil {
			push.InitStreamAndGroup(
				config.Conf.GatewayDispatcherConfig.StreamName,
				config.Conf.GatewayDispatcherConfig.GroupName,
				config.Conf.GatewayDispatcherConfig.ConsumerName,
				time.Duration(config.Conf.GatewayDispatcherConfig.Interval)*time.Second,
				config.Conf.GatewayDispatcherConfig.ThresholdPending)
			zap.L().Info("Initialized stream and group for push dispatcher")
		}
	} else {
		zap.L().Warn("Registry URL not configured, gateway will not register")
	}

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

	// Stop heartbeat
	if heartbeatStopCh != nil {
		close(heartbeatStopCh)
	}

	// Unregister from Registry
	if registryClient != nil {
		if err := registryClient.Unregister(); err != nil {
			zap.L().Error("Failed to unregister from registry", zap.Error(err))
		} else {
			zap.L().Info("Gateway unregistered from registry")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		zap.L().Error("server shutdown error", zap.Error(err))
	}

	zap.L().Info("gateway server exited")
}
