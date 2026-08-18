package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HTTPMirrorServerConfig struct {
	CacheDir      string
	UpstreamURL   string
	ListenAddress string
}

type HTTPMirrorServer struct {
	cacheDir    string
	upstreamURL string
	server      *http.Server
	listener    net.Listener
	HostURL     string `json:"hostUrl"`
	GuestURL    string `json:"guestUrl"`
	CacheDir    string `json:"cacheDir"`
	UpstreamURL string `json:"upstreamUrl"`
}

type cachedResponseMeta struct {
	RequestURI  string    `json:"requestUri"`
	ContentType string    `json:"contentType"`
	CachedAt    time.Time `json:"cachedAt"`
}

func StartHTTPMirrorServer(ctx context.Context, cfg HTTPMirrorServerConfig) (*HTTPMirrorServer, error) {
	if strings.TrimSpace(cfg.CacheDir) == "" {
		return nil, fmt.Errorf("cache directory is required")
	}
	if strings.TrimSpace(cfg.UpstreamURL) == "" {
		return nil, fmt.Errorf("upstream url is required")
	}
	if cfg.ListenAddress == "" {
		cfg.ListenAddress = "0.0.0.0:0"
	}

	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}

	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		return nil, fmt.Errorf("listen for cache server: %w", err)
	}

	server := &HTTPMirrorServer{
		cacheDir:    cfg.CacheDir,
		upstreamURL: strings.TrimRight(cfg.UpstreamURL, "/"),
		listener:    listener,
		CacheDir:    cfg.CacheDir,
		UpstreamURL: strings.TrimRight(cfg.UpstreamURL, "/"),
	}
	server.server = &http.Server{Handler: server, ReadHeaderTimeout: 10 * time.Second}

	port := listener.Addr().(*net.TCPAddr).Port
	server.HostURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	server.GuestURL = fmt.Sprintf("http://10.0.2.2:%d", port)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.server.Shutdown(shutdownCtx)
	}()

	go func() {
		_ = server.server.Serve(listener)
	}()

	return server, nil
}

func (server *HTTPMirrorServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyPath, metaPath := server.cachePaths(request.URL.RequestURI())
	if meta, ok := server.loadMeta(metaPath); ok {
		if _, err := os.Stat(bodyPath); err == nil {
			server.serveCached(writer, request, bodyPath, meta)
			return
		}
	}

	server.fetchAndCache(writer, request, bodyPath, metaPath)
}

func (server *HTTPMirrorServer) serveCached(writer http.ResponseWriter, request *http.Request, bodyPath string, meta cachedResponseMeta) {
	if meta.ContentType != "" {
		writer.Header().Set("Content-Type", meta.ContentType)
	}
	writer.Header().Set("X-Skea-Cache", "HIT")

	file, err := os.Open(bodyPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	if request.Method == http.MethodHead {
		writer.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		writer.WriteHeader(http.StatusOK)
		return
	}

	http.ServeContent(writer, request, filepath.Base(bodyPath), info.ModTime(), file)
}

func (server *HTTPMirrorServer) fetchAndCache(writer http.ResponseWriter, request *http.Request, bodyPath, metaPath string) {
	targetURL := server.upstreamURL + request.URL.RequestURI()
	upstreamRequest, err := http.NewRequestWithContext(request.Context(), http.MethodGet, targetURL, nil)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	response, err := defaultHTTPClient.Do(upstreamRequest)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadGateway)
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		writer.WriteHeader(response.StatusCode)
		_, _ = io.Copy(writer, response.Body)
		return
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(bodyPath), 0o755); mkdirErr != nil {
		http.Error(writer, mkdirErr.Error(), http.StatusInternalServerError)
		return
	}

	tmpBodyPath := bodyPath + ".tmp"
	file, err := os.Create(tmpBodyPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		_ = file.Close()
		_ = os.Remove(tmpBodyPath)
		http.Error(writer, err.Error(), http.StatusBadGateway)
		return
	}

	if _, writeErr := file.Write(body); writeErr != nil {
		_ = file.Close()
		_ = os.Remove(tmpBodyPath)
		http.Error(writer, writeErr.Error(), http.StatusInternalServerError)
		return
	}
	if closeErr := file.Close(); closeErr != nil {
		_ = os.Remove(tmpBodyPath)
		http.Error(writer, closeErr.Error(), http.StatusInternalServerError)
		return
	}

	if renameErr := os.Rename(tmpBodyPath, bodyPath); renameErr != nil {
		_ = os.Remove(tmpBodyPath)
		http.Error(writer, renameErr.Error(), http.StatusInternalServerError)
		return
	}

	meta := cachedResponseMeta{
		RequestURI:  request.URL.RequestURI(),
		ContentType: response.Header.Get("Content-Type"),
		CachedAt:    time.Now().UTC(),
	}
	metaBlob, err := json.MarshalIndent(meta, "", "  ")
	if err == nil {
		_ = os.WriteFile(metaPath, metaBlob, 0o600)
	}

	if meta.ContentType != "" {
		writer.Header().Set("Content-Type", meta.ContentType)
	}
	writer.Header().Set("X-Skea-Cache", "MISS")
	writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	writer.WriteHeader(http.StatusOK)
	if request.Method != http.MethodHead {
		_, _ = writer.Write(body)
	}
}

func (server *HTTPMirrorServer) loadMeta(metaPath string) (cachedResponseMeta, bool) {
	blob, err := os.ReadFile(metaPath)
	if err != nil {
		return cachedResponseMeta{}, false
	}

	var meta cachedResponseMeta
	if err := json.Unmarshal(blob, &meta); err != nil {
		return cachedResponseMeta{}, false
	}

	return meta, true
}

func (server *HTTPMirrorServer) cachePaths(requestURI string) (string, string) {
	hash := sha256.Sum256([]byte(requestURI))
	key := hex.EncodeToString(hash[:])
	dir := filepath.Join(server.cacheDir, key[:2])
	return filepath.Join(dir, key+".body"), filepath.Join(dir, key+".json")
}
