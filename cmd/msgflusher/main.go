package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"GoStacker/internal/msgflusher"
	"GoStacker/pkg/config"
	rdb "GoStacker/pkg/db/redis"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/monitor"
)

func main() {
	cfgPath := flag.String("config", "config.msgflusher.yaml", "path to msgflusher config yaml")
	flag.Parse()

	if err := config.InitFromFile(*cfgPath); err != nil {
		fmt.Printf("init config failed: %v\n", err)
		os.Exit(1)
	}
	if err := logger.Init(config.Conf.LogConfig); err != nil {
		fmt.Printf("init logger failed: %v\n", err)
		os.Exit(1)
	}
	if err := rdb.Init(config.Conf.RedisConfig); err != nil {
		fmt.Printf("init redis failed: %v\n", err)
		os.Exit(1)
	}
	defer rdb.Close()
	monitor.InitMonitor()

	reclaimer, err := msgflusher.NewReclaimer(config.Conf)
	if err != nil {
		fmt.Printf("init msgflusher reclaimer failed: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	reclaimer.Run(ctx)
}
