package main

import (
	"GoStacker/internal/send/route"
	"GoStacker/pkg/bootstrap"
	"GoStacker/pkg/config"
	"GoStacker/pkg/push"
	"GoStacker/pkg/registry_client"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

var registryClient *registry_client.SendClient
var heartbeatStopCh chan struct{}

func main() {
	cfgPath := flag.String("config", "config.send.yaml", "path to config yaml")
	flag.Parse()

	cleanup, err := bootstrap.InitAll(*cfgPath)
	if err != nil {
		fmt.Printf("bootstrap init failed: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	// Initialize Registry client if configured
	if config.Conf != nil && config.Conf.RegistryConfig.URL != "" {
		instanceID := fmt.Sprintf("send-%s-%d", config.Conf.Name, config.Conf.MachineID)
		registryClient = registry_client.NewSendClient(config.Conf.RegistryConfig.URL, instanceID)

		// Register with Registry service
		err = registryClient.Register(config.Conf.Address, config.Conf.Port)
		if err != nil {
			zap.L().Error("Failed to register with registry service", zap.Error(err))
			fmt.Printf("Failed to register with registry: %v\n", err)
			// Continue anyway, will retry on heartbeat
		} else {
			zap.L().Info("Send instance registered with registry",
				zap.String("instance_id", instanceID),
				zap.String("address", config.Conf.Address),
				zap.Int("port", config.Conf.Port))
		}

		// Start heartbeat
		heartbeatStopCh = registryClient.StartHeartbeat(10 * time.Second)
		defer func() {
			close(heartbeatStopCh)
			// Unregister on shutdown
			if err := registryClient.Unregister(); err != nil {
				zap.L().Error("Failed to unregister from registry", zap.Error(err))
			} else {
				zap.L().Info("Send instance unregistered from registry")
			}
		}()

		// Initialize route service with registry client
		route.InitRouteService(registryClient)
	} else {
		zap.L().Warn("Registry URL not configured, route cache will not be available")
	}

	// Initialize push-related components based on PushMod
	if config.Conf != nil {
		if config.Conf.PushMod == "standalone" {
			push.InitDispatcher(config.Conf.SendDispatcherConfig)
		} else if config.Conf.PushMod == "gateway" {
			// start gateway dispatcher worker pool
			gwWorkers := config.Conf.SendDispatcherConfig.GatewayWorkerCount
			gwQueue := config.Conf.SendDispatcherConfig.GatewayQueueSize
			if gwWorkers <= 0 {
				gwWorkers = 1
			}
			if gwQueue <= 0 {
				gwQueue = 1024
			}
			push.StartGatewayDispatcher(gwWorkers, gwQueue)
		}
	}

	// build send service router and start server on configured port
	engine := NewRouter()
	addr := fmt.Sprintf(":%d", config.Conf.Port)
	serverSrv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	zap.L().Info("Send service starting", zap.String("address", addr))
	fmt.Printf("send service listening on %s\n", addr)

	if err := serverSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		zap.L().Error("Send service listen failed", zap.Error(err))
		fmt.Printf("send service listen failed: %v\n", err)
		os.Exit(1)
	}
}
