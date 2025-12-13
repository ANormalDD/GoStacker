package main

import (
	"GoStacker/internal/meta/chat/group"
	chatsend "GoStacker/internal/send/chat/send"
	"GoStacker/pkg/bootstrap"
	"GoStacker/pkg/config"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	cfgPath := flag.String("config", "config.flusher.yaml", "path to config yaml")
	flag.Parse()

	cleanup, err := bootstrap.InitAll(*cfgPath)
	if err != nil {
		fmt.Printf("bootstrap init failed: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	// determine flusher interval and batch from config
	interval := 5 * time.Second
	batch := 100
	if config.Conf != nil && config.Conf.GroupCacheConfig != nil {
		if config.Conf.GroupCacheConfig.FlushIntervalSeconds > 0 {
			interval = time.Duration(config.Conf.GroupCacheConfig.FlushIntervalSeconds) * time.Second
		}
		if config.Conf.GroupCacheConfig.BatchSize > 0 {
			batch = config.Conf.GroupCacheConfig.BatchSize
		}
	}

	stopCh := make(chan struct{})
	go group.RunGroupFlusher(interval, batch, stopCh)
	go chatsend.StartMessageFlusher(5*time.Second, 100, stopCh)

	// block forever
	select {}
}
