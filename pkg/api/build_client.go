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

	"distributed_build/pkg/build"
)

type BuildClient struct {
	logger   *zap.Logger
	endpoint string
	client   *http.Client
}

func NewBuildClient(l *zap.Logger, endpoint string) *BuildClient {
	return &BuildClient{logger: l, endpoint: endpoint, client: http.DefaultClient}
}

func (c *BuildClient) StartBuild(ctx context.Context, request *BuildRequest) (*BuildStarted, StatusReader, error) {
	reqJSON, err := json.Marshal(request)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/build", bytes.NewBuffer(reqJSON))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errorData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read error response: %w", err)
		}
		return nil, nil, fmt.Errorf("service error: %s", string(errorData))
	}
	var buildStarted BuildStarted
	if err := json.NewDecoder(resp.Body).Decode(&buildStarted); err != nil {
		return nil, nil, fmt.Errorf("failed to decode build started response: %w", err)
	}
	return &buildStarted, NewStatusReader(resp.Body), nil

}

func (c *BuildClient) SignalBuild(ctx context.Context, buildID build.ID, signal *SignalRequest) (*SignalResponse, error) {
	signalJSON, err := json.Marshal(signal)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signal request: %w", err)
	}
	buildIDText := buildID.String()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/signal?build_id=%s", c.endpoint, string(buildIDText)), bytes.NewBuffer(signalJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create signal request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("signal request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read error response: %w", err)
		}
		return nil, fmt.Errorf("service error: %s", string(errorData))
	}

	var signalResponse SignalResponse
	if err := json.NewDecoder(resp.Body).Decode(&signalResponse); err != nil {
		return nil, fmt.Errorf("failed to decode signal response: %w", err)
	}

	return &signalResponse, nil
}

type statusReader struct {
	r io.ReadCloser
}

func NewStatusReader(reader io.ReadCloser) StatusReader {
	return &statusReader{r: reader}
}
func (sr *statusReader) Next() (*StatusUpdate, error) {
	var update StatusUpdate
	if err := json.NewDecoder(sr.r).Decode(&update); err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("failed to decode status update: %w", err)
	}
	return &update, nil
}

func (sr *statusReader) Close() error {
	sr.r.Close()
	return nil
}
