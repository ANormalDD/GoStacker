package centerclient

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
)

var centralAddr string
var gatewayID string

// Init initializes center client with central server base URL (e.g. http://host:port)
// and this gateway's id string used when registering users.
func Init(centralBaseURL string, gwID string) {
	centralAddr = centralBaseURL
	gatewayID = gwID
}

func post(path string, body interface{}) error {
	if centralAddr == "" {
		return nil
	}
	b, _ := json.Marshal(body)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(centralAddr+path, "application/json", bytes.NewReader(b))
	if err != nil {
		zap.L().Warn("centerclient http request failed", zap.Error(err))
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		zap.L().Warn("centerclient non-2xx response", zap.Int("code", resp.StatusCode))
	}
	return nil
}

// RegisterUser notifies central server that userID is connected to this gateway
func RegisterUser(userID int64) error {
	return post("/register", map[string]interface{}{"user_id": userID, "gateway_id": gatewayID})
}

// UnregisterUser notifies central server that userID is disconnected from this gateway
func UnregisterUser(userID int64) error {
	return post("/unregister", map[string]interface{}{"user_id": userID, "gateway_id": gatewayID})
}

// RegisterGateway registers this gateway id and address to central server
func RegisterGateway(gatewayAddr string) error {
	return post("/register", map[string]interface{}{"gateway_id": gatewayID, "gateway_addr": gatewayAddr})
}

// QueryUser asks central server which gateway a user is connected to.
// returns gatewayID, gatewayAddr, found(bool), error
func QueryUser(userID int64) (string, string, bool, error) {
	if centralAddr == "" {
		return "", "", false, nil
	}
	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest("GET", centralAddr+"/query?user_id="+strconv.FormatInt(userID, 10), nil)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", false, nil
	}
	var data map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", false, err
	}
	return data["gateway_id"], data["gateway_addr"], true, nil
}

// ForwardToGateway POSTs payload (raw marshaled message) to target gateway's /forward endpoint
func ForwardToGateway(gatewayAddr string, payload []byte) error {
	if gatewayAddr == "" {
		return nil
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(gatewayAddr+"/forward", "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// ReportLoad sends current load metrics (queueLen and queueCap) to central server
func ReportLoad(queueLen int, queueCap int) error {
	return post("/report", map[string]interface{}{"gateway_id": gatewayID, "queue_len": queueLen, "queue_cap": queueCap})
}

// SelectGateway asks central server to pick a low-load gateway. Returns gateway_id and gateway_addr if found.
func SelectGateway() (string, string, bool, error) {
	if centralAddr == "" {
		return "", "", false, nil
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(centralAddr + "/select_gateway")
	if err != nil {
		return "", "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", false, nil
	}
	var data map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", false, err
	}
	return data["gateway_id"], data["gateway_addr"], true, nil
}

// Handoff sends an array of pending tasks to central server for safe persistence/redistribution.
// payload should be map[string]interface{}{"gateway_id": gatewayID, "tasks": []map[string]interface{}{...}}
func Handoff(payload interface{}) error {
	return post("/handoff", payload)
}

// GatewayID returns configured gateway id
func GatewayID() string {
	return gatewayID
}

// HandoffPaged sends tasks in pages, each page gzipped and posted to /handoff
func HandoffPaged(tasks []map[string]interface{}, pageSize int) error {
	if pageSize <= 0 {
		pageSize = 100
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for i := 0; i < len(tasks); i += pageSize {
		end := i + pageSize
		if end > len(tasks) {
			end = len(tasks)
		}
		page := tasks[i:end]
		// build payload
		payload := map[string]interface{}{"gateway_id": gatewayID, "tasks": page}
		b, _ := json.Marshal(payload)
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(b); err != nil {
			gw.Close()
			return err
		}
		gw.Close()
		req, _ := http.NewRequest("POST", centralAddr+"/handoff", &buf)
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/gzip")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			return nil
		}
	}
	return nil
}
