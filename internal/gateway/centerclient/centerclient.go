package centerclient

// centerclient provides a small indirection layer so other packages (eg. pkg/push)
// can call into the center client without importing the top-level center_client package,
// which would create an import cycle.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"GoStacker/internal/gateway/push/types"
	"GoStacker/pkg/config"
	"GoStacker/pkg/registry_client"

	"go.uber.org/zap"
)

// SendPushBackFunc is the function signature used to send a pushback request to center.
type SendPushBackFunc func(cfg *config.CenterConfig, forwardReq types.ClientMessage, targetID int64) error

var (
	// default implementation will query Registry for send instances and POST to /internal/pushback
	sendPushBackFunc SendPushBackFunc = func(cfg *config.CenterConfig, forwardReq types.ClientMessage, targetID int64) error {
		// Determine registry URL from global config if available
		if config.Conf == nil || config.Conf.RegistryConfig == nil || config.Conf.RegistryConfig.URL == "" {
			return errors.New("registry url not configured")
		}

		sc := registry_client.NewSendClient(config.Conf.RegistryConfig.URL, "gateway-pushback-client")
		instances, err := sc.ListSendInstances()
		if err != nil {
			zap.L().Error("failed to list send instances from registry", zap.Error(err))
			return err
		}
		if len(instances) == 0 {
			err := errors.New("no send instances available")
			zap.L().Warn("no send instances returned from registry")
			return err
		}

		// choose a random instance
		rand.Seed(time.Now().UnixNano())
		idx := rand.Intn(len(instances))
		inst := instances[idx]

		// build pushback request body
		body := map[string]interface{}{
			"target_id":   targetID,
			"forward_req": forwardReq,
		}
		data, err := json.Marshal(body)
		if err != nil {
			zap.L().Error("failed to marshal pushback request", zap.Error(err))
			return err
		}

		url := fmt.Sprintf("http://%s:%d/internal/pushback", inst.Address, inst.Port)
		client := &http.Client{Timeout: 3 * time.Second}

		// try with limited retries
		var lastErr error
		for attempt := 0; attempt < 3; attempt++ {
			req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				lastErr = err
				zap.L().Warn("pushback http request failed", zap.String("url", url), zap.Int("attempt", attempt), zap.Error(err))
				time.Sleep(100 * time.Millisecond)
				continue
			}
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				zap.L().Debug("pushback delivered to send instance", zap.String("url", url), zap.String("instance", inst.InstanceID))
				return nil
			}
			lastErr = fmt.Errorf("unexpected status %d", resp.StatusCode)
			zap.L().Warn("pushback http request returned unexpected status", zap.Int("status", resp.StatusCode), zap.String("url", url))
			time.Sleep(100 * time.Millisecond)
		}
		return lastErr
	}
)

// RegisterSender registers the concrete function used to perform SendPushBackRequest.
// Typically called by the `center_client` package during startup.
func RegisterSender(f SendPushBackFunc) {
	if f == nil {
		return
	}
	sendPushBackFunc = f
}

// SendPushBackRequest delegates to the registered sender implementation.
func SendPushBackRequest(cfg *config.CenterConfig, forwardReq types.ClientMessage, targetID int64) error {
	return sendPushBackFunc(cfg, forwardReq, targetID)
}
