package main

import (
	"GoStacker/pkg/bootstrap"
	"GoStacker/pkg/config"
	"flag"
	"fmt"
	"net/http"
	"os"
)

func main() {
	cfgPath := flag.String("config", "config.meta.yaml", "path to config yaml")
	flag.Parse()

	cleanup, err := bootstrap.InitAll(*cfgPath)
	if err != nil {
		fmt.Printf("bootstrap init failed: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	engine := NewRouter()
	addr := fmt.Sprintf(":%d", config.Conf.Port)
	srv := &http.Server{Addr: addr, Handler: engine}
	fmt.Printf("meta service listening on %s\n", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Printf("meta service listen failed: %v\n", err)
		os.Exit(1)
	}
}
