package getGatewayAddr

import (
	"GoStacker/internal/gateway"
	"GoStacker/pkg/response"
	"GoStacker/pkg/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func GetGatewayAddrHandler(c *gin.Context) {
	//get the lowest load gateway
	if config.Conf.Mode != "gateway" {
		response.ReplySuccessWithData(c, "standalone", gin.H{
			"gateway_id": "standalone",
			"address":    config.Conf.Address,
		})
		return
	}
	gatewayItem, ok := gateway.DefaultManager.Peek()
	if !ok {
		zap.L().Error("No available gateway")
		response.ReplyError500(c, "No available gateway")
		return
	}
	response.ReplySuccessWithData(c, "available", gin.H{
		"gateway_id": gatewayItem.GatewayID,
		"address":    gatewayItem.Address,
	})

}
