//go:build !solution

package artifact

import (
	"context"

	"distributed_build/pkg/build"
)

// Download artifact from remote cache into local cache.
func Download(ctx context.Context, endpoint string, c *Cache, artifactID build.ID) error {
	panic("implement me")
}
