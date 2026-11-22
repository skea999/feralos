package cache

// FeralosRepoServer serves the staged aports artifact directory (the repo
// layout: feralos-aports/x86_64 + feralos.pub) as plain static files. The
// Alpine cache mirror cannot serve it: it proxies every path to the upstream
// (dl-cdn), so a feralos path would 404 (keyring wget failure, run
// 2026-11-22T00-25-46Z).

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const headerTimeout = 30 * time.Second

type FeralosRepoServer struct {
	listener net.Listener
	GuestURL string `json:"guestUrl"`
}

// StartFeralosRepoServer serves dir (the staged artifact) and returns a
// server whose GuestURL is reachable from the guest (10.0.2.2). The listener
// closes when ctx is done.
func StartFeralosRepoServer(ctx context.Context, dir string) (*FeralosRepoServer, error) {
	indexPath := filepath.Join(dir, "feralos-aports", "x86_64", "APKINDEX.tar.gz")
	if _, err := os.Stat(indexPath); err != nil {
		return nil, fmt.Errorf("feralos repo dir incomplete: %w", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	server := &FeralosRepoServer{listener: listener}
	server.GuestURL = fmt.Sprintf("http://10.0.2.2:%d", listener.Addr().(*net.TCPAddr).Port)
	httpSrv := &http.Server{Handler: http.FileServer(http.Dir(dir)), ReadHeaderTimeout: headerTimeout}
	go func() {
		_ = httpSrv.Serve(listener)
	}()
	go func() {
		<-ctx.Done()
		_ = httpSrv.Close()
	}()
	return server, nil
}
