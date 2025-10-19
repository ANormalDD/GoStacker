package gateway

import (
	"GoStacker/pkg/config"
	"GoStacker/pkg/db/redis"
	"GoStacker/push_distribute/gateway/centerclient"
	"GoStacker/push_distribute/gateway/push"
	"GoStacker/push_distribute/gateway/server"
	"GoStacker/push_distribute/gateway/ws"
	"fmt"
	"time"

	"go.uber.org/zap"
)

func Run() {
	// load local config file if exists
	// For simplicity reuse global config if initialized; otherwise minimal defaults
	if err := config.Init(); err != nil {
		fmt.Println("Failed to init config", err)
	}
	// init redis
	if err := redis.Init(config.Conf.RedisConfig); err != nil {
		zap.L().Warn("Failed to init redis", zap.Error(err))
	}
	// init center client
	centralBase := fmt.Sprintf("http://%s:%d", "localhost", 9090)
	centerclient.Init(centralBase, "gateway-1")
	// register gateway addr
	_ = centerclient.RegisterGateway("http://localhost:8080")

	// periodic load reporter
	go func() {
		for {
			queueLen := len(push.PushTaskChan)
			queueCap := cap(push.PushTaskChan)
			_ = centerclient.ReportLoad(queueLen, queueCap)
			time.Sleep(2 * time.Second)
		}
	}()

	// init dispatcher
	push.InitDispatcher(config.Conf.DispatcherConfig)

	// setup router and register ws handler
	r := server.NewRouter()
	// mount ws handler; assuming authentication middleware sets userID in context
	r.GET("/ws", ws.WebSocketHandler)

	zap.L().Info("Starting gateway http server on :8080 (gin)")
	r.Run(":8080")
}
