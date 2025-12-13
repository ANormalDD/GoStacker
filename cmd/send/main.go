package main

import (
	"GoStacker/internal/send/gateway/mid"
	"GoStacker/pkg/bootstrap"
	"GoStacker/pkg/config"
	"GoStacker/pkg/push"
	"flag"
	"fmt"
	"net/http"
	"os"
)

func main() {
	cfgPath := flag.String("config", "config.send.yaml", "path to config yaml")
	flag.Parse()

	cleanup, err := bootstrap.InitAll(*cfgPath)
	if err != nil {
		fmt.Printf("bootstrap init failed: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	// Initialize push-related components based on PushMod
	if config.Conf != nil {
		if config.Conf.PushMod == "standalone" {
			push.InitDispatcher(config.Conf.SendDispatcherConfig)
		} else if config.Conf.PushMod == "gateway" {
			// start gateway dispatcher worker pool

			gwWorkers := config.Conf.SendDispatcherConfig.GatewayWorkerCount
			gwQueue := config.Conf.SendDispatcherConfig.GatewayQueueSize
			mid.RegisterPushOfflineMessagesFuc(push.PushOfflineMessages)
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
	fmt.Printf("send service listening on %s\n", addr)
	if err := serverSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Printf("send service listen failed: %v\n", err)
		os.Exit(1)
	}
}
