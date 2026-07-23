package cache

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var defaultHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext(ctx, "tcp4", addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
	Timeout: 5 * time.Minute,
}

func EnsureDownloaded(ctx context.Context, url, dstPath string) (string, error) {
	if strings.TrimSpace(url) == "" {
		return "", fmt.Errorf("download url is required")
	}

	if _, err := os.Stat(dstPath); err == nil {
		return dstPath, nil
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return "", fmt.Errorf("create cache directory: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s returned %s", url, resp.Status)
	}

	tmpPath := dstPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", tmpPath, err)
	}

	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		return "", fmt.Errorf("write %s: %w", tmpPath, err)
	}

	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, dstPath); err != nil {
		return "", fmt.Errorf("rename %s to %s: %w", tmpPath, dstPath, err)
	}

	return dstPath, nil
}
