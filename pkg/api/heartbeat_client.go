//go:build !solution

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"
)

type HeartbeatClient struct {
	logger   *zap.Logger
	endpoint string
	client   *http.Client
}

func NewHeartbeatClient(l *zap.Logger, endpoint string) *HeartbeatClient {
	return &HeartbeatClient{logger: l, endpoint: endpoint + "/heartbeat", client: http.DefaultClient}
}
func (c *HeartbeatClient) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	body := bytes.NewBuffer(reqJSON)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		errorData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read error response: %w", err)
		}

		return nil, fmt.Errorf("error response: %s", string(errorData))
	}

	// Decode the successful response
	var heartbeatResponse HeartbeatResponse
	if err := json.NewDecoder(resp.Body).Decode(&heartbeatResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &heartbeatResponse, nil
}
