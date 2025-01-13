//go:build !solution

package filecache

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"go.uber.org/zap"

	"distributed_build/pkg/build"
)

type Client struct {
	client   *http.Client
	logger   *zap.Logger
	endpoint string
}

func NewClient(l *zap.Logger, endpoint string) *Client {
	return &Client{
		client:   http.DefaultClient,
		logger:   l,
		endpoint: endpoint,
	}
}
func (c *Client) Upload(ctx context.Context, id build.ID, localPath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		c.logger.Error("failed to open local file", zap.String("path", localPath), zap.Error(err))
		return err
	}
	defer file.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.endpoint+"/file?id="+id.String(), file)
	if err != nil {
		c.logger.Error("failed to create request", zap.Error(err))
		return err
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("failed to perform request", zap.Error(err))
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errorData, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read error response: %w", err)
		}
		return fmt.Errorf("upload failed with status: %s", string(errorData))
	}

	c.logger.Info("file uploaded successfully", zap.String("id", id.String()))
	return nil
}

func (c *Client) Download(ctx context.Context, localCache *Cache, id build.ID) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/file?id="+id.String(), nil)
	if err != nil {
		c.logger.Error("failed to create request", zap.Error(err))
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("failed to perform request", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorData, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read error response: %w", err)
		}
		return fmt.Errorf("download failed with status: %s", string(errorData))
	}

	writer, abort, err := localCache.Write(id)
	if err != nil {
		c.logger.Error("failed to get writer for local cache", zap.Error(err))
		return err
	}
	defer writer.Close()
	if _, err := io.Copy(writer, resp.Body); err != nil {
		c.logger.Error("failed write to local cache", zap.Error(err))
		abort()
		return err
	}
	return nil

}
