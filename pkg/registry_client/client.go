package registry_client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

var (
	ErrRequestFailed   = errors.New("registry request failed")
	ErrInvalidResponse = errors.New("invalid response from registry")
)

// Client is a client for interacting with Registry service
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new registry client
func NewClient(registryURL string) *Client {
	return &Client{
		baseURL: registryURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Response represents a generic API response
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// doRequest performs an HTTP request with retry logic
func (c *Client) doRequest(method, path string, body interface{}, maxRetries int) (*Response, error) {
	var lastErr error
	backoff := 100 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			zap.L().Warn("Retrying registry request",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("attempt", attempt))
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 5*time.Second {
				backoff = 5 * time.Second
			}
		}

		resp, err := c.doSingleRequest(method, path, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries+1, lastErr)
}

// doSingleRequest performs a single HTTP request
func (c *Client) doSingleRequest(method, path string, body interface{}) (*Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var resp Response
	err = json.Unmarshal(respBody, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", httpResp.StatusCode, resp.Message)
	}

	return &resp, nil
}

// Ping checks if registry service is available
func (c *Client) Ping() error {
	resp, err := c.doSingleRequest("GET", "/ping", nil)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("ping failed: %s", resp.Message)
	}
	return nil
}
