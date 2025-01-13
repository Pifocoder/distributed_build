//go:build !solution

package artifact

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"distributed_build/pkg/build"
	"distributed_build/pkg/tarstream"
)

// Download artifact from remote cache into local cache.
func Download(ctx context.Context, endpoint string, c *Cache, artifactID build.ID) error {
	artifactIDText := artifactID.String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/artifact?id=%s", endpoint, artifactIDText), nil)
	if err != nil {
		return fmt.Errorf("failed to create get artifact request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("get artifact request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorData, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read error response: %w", err)
		}
		return fmt.Errorf("service error: %s", string(errorData))
	}
	path, commit, abort, err := c.Create(artifactID)
	if err != nil {
		return fmt.Errorf("unable to create artifact storage: %w", err)
	}
	err = tarstream.Receive(path, resp.Body)
	if err != nil {
		abort()
		return fmt.Errorf("unable to save artifact: %w", err)
	}
	return commit()
}
