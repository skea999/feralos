package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"skeaAlpineInstaller/tests/harness/internal/service"
)

type Server struct {
	service       *service.Service
	repoRoot      string
	artifactsRoot string
	mu            sync.Mutex
	jobs          map[string]*Job
}

type Job struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
	Result     any       `json:"result,omitempty"`
	Error      string    `json:"error,omitempty"`
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewServer(harnessService *service.Service, repoRoot, artifactsRoot string) *Server {
	return &Server{service: harnessService, repoRoot: repoRoot, artifactsRoot: artifactsRoot, jobs: map[string]*Job{}}
}

func (server *Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	decoder := json.NewDecoder(input)
	encoder := json.NewEncoder(output)

	for {
		var req request
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("decode MCP request: %w", err)
		}

		resp := server.handleRequest(ctx, req)
		if resp == nil || len(req.ID) == 0 {
			continue
		}

		if err := encoder.Encode(resp); err != nil {
			return fmt.Errorf("encode MCP response: %w", err)
		}
	}
}

func (server *Server) handleRequest(ctx context.Context, req request) *response {
	switch req.Method {
	case "initialize":
		return &response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "skea-harness-mcp", "version": "0.1.0"},
		}}
	case "notifications/initialized":
		return nil
	case "tools/list":
		return &response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": server.toolDefinitions()}}
	case "tools/call":
		return server.handleToolCall(ctx, req)
	default:
		return &response{JSONRPC: "2.0", ID: req.ID, Error: &responseError{Code: -32601, Message: "method not found"}}
	}
}

func (server *Server) handleToolCall(ctx context.Context, req request) *response {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &response{JSONRPC: "2.0", ID: req.ID, Error: &responseError{Code: -32602, Message: err.Error()}}
	}

	toolResponse := func(result any, isError bool) *response {
		blob, _ := json.MarshalIndent(result, "", "  ")
		return &response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"content":           []map[string]any{{"type": "text", "text": string(blob)}},
			"structuredContent": result,
			"isError":           isError,
		}}
	}

	switch params.Name {
	case "prepare_runner":
		cfg := service.PrepareRunnerConfig{
			RepoRoot:        server.repoRoot,
			ArtifactsRoot:   stringArg(params.Arguments, "artifactsRoot", server.artifactsRoot),
			CacheDir:        stringArg(params.Arguments, "cacheDir", filepath.Join(server.artifactsRoot, "cache")),
			RunnerImage:     stringArg(params.Arguments, "runnerImage", filepath.Join(server.artifactsRoot, "cache", "runner-images", "skea-runner.qcow2")),
			BaseImageURL:    stringArg(params.Arguments, "baseImageUrl", ""),
			BaseImagePath:   stringArg(params.Arguments, "baseImagePath", ""),
			QemuBinary:      stringArg(params.Arguments, "qemuBinary", "qemu-system-x86_64"),
			QemuImgBinary:   stringArg(params.Arguments, "qemuImgBinary", "qemu-img"),
			CloudLocaldsBin: stringArg(params.Arguments, "cloudLocaldsBinary", "cloud-localds"),
			SSHPort:         intArg(params.Arguments, "sshPort", 2222),
			SSHUser:         stringArg(params.Arguments, "sshUser", "skea"),
			MemoryMiB:       intArg(params.Arguments, "memory", 4096),
			CPUs:            intArg(params.Arguments, "cpus", 2),
			EnableKVM:       boolArg(params.Arguments, "enableKvm", true),
			Force:           boolArg(params.Arguments, "force", false),
			AptProxy:        stringArg(params.Arguments, "aptProxy", ""),
			BootTimeout:     time.Duration(intArg(params.Arguments, "bootTimeoutSeconds", 300)) * time.Second,
		}
		result, err := server.service.PrepareRunner(ctx, cfg)
		if err != nil {
			return toolResponse(map[string]any{"error": err.Error()}, true)
		}
		return toolResponse(result, false)
	case "run_install_async":
		job := server.newJob("run-install")
		cfg := service.RunConfig{
			RepoRoot:            stringArg(params.Arguments, "repoRoot", server.repoRoot),
			ArtifactsRoot:       stringArg(params.Arguments, "artifactsRoot", server.artifactsRoot),
			RunnerImage:         stringArg(params.Arguments, "runnerImage", filepath.Join(server.artifactsRoot, "cache", "runner-images", "skea-runner.qcow2")),
			QemuBinary:          stringArg(params.Arguments, "qemuBinary", "qemu-system-x86_64"),
			QemuImgBinary:       stringArg(params.Arguments, "qemuImgBinary", "qemu-img"),
			RunnerSSHUser:       stringArg(params.Arguments, "runnerSshUser", "skea"),
			RunnerSSHPort:       intArg(params.Arguments, "runnerSshPort", 2222),
			TargetSSHPort:       intArg(params.Arguments, "targetSshPort", 2223),
			MemoryMiB:           intArg(params.Arguments, "memory", 4096),
			TargetMemoryMiB:     intArg(params.Arguments, "targetMemory", 4096),
			CPUs:                intArg(params.Arguments, "cpus", 2),
			EnableKVM:           boolArg(params.Arguments, "enableKvm", true),
			EnableTPM:           boolArg(params.Arguments, "enableTpm", false),
			DiskSize:            stringArg(params.Arguments, "diskSize", "16G"),
			AlpineMirror:        stringArg(params.Arguments, "alpineMirror", ""),
			InstalledUser:       stringArg(params.Arguments, "installedUser", "skea"),
			InstalledPassword:   stringArg(params.Arguments, "installedPassword", "Alpine.1"),
			KeepTargetRunning:   boolArg(params.Arguments, "keepTargetRunning", false),
			BootTimeout:         time.Duration(intArg(params.Arguments, "bootTimeoutSeconds", 300)) * time.Second,
			VerificationTimeout: time.Duration(intArg(params.Arguments, "verifyTimeoutSeconds", 120)) * time.Second,
		}

		go server.runAsyncJob(ctx, job.ID, cfg)
		return toolResponse(map[string]any{"jobId": job.ID, "status": job.Status}, false)
	case "get_status":
		status, err := server.service.Status(ctx, stringArg(params.Arguments, "artifactsRoot", server.artifactsRoot))
		if err != nil {
			return toolResponse(map[string]any{"error": err.Error()}, true)
		}
		return toolResponse(map[string]any{"service": status, "jobs": server.jobSnapshot()}, false)
	case "stop_active_run":
		result, err := server.service.Stop(ctx, service.StopConfig{ArtifactsRoot: stringArg(params.Arguments, "artifactsRoot", server.artifactsRoot), Force: boolArg(params.Arguments, "force", false)})
		if err != nil {
			return toolResponse(map[string]any{"error": err.Error()}, true)
		}
		return toolResponse(result, false)
	case "cleanup_stale":
		result, err := server.service.Cleanup(ctx, service.CleanupConfig{ArtifactsRoot: stringArg(params.Arguments, "artifactsRoot", server.artifactsRoot), Force: boolArg(params.Arguments, "force", false)})
		if err != nil {
			return toolResponse(map[string]any{"error": err.Error()}, true)
		}
		return toolResponse(result, false)
	case "tail_log":
		result, err := server.service.Logs(ctx, service.LogsConfig{ArtifactsRoot: stringArg(params.Arguments, "artifactsRoot", server.artifactsRoot), RunDir: stringArg(params.Arguments, "runDir", ""), LogName: stringArg(params.Arguments, "log", "installer"), Lines: intArg(params.Arguments, "lines", 200)})
		if err != nil {
			return toolResponse(map[string]any{"error": err.Error()}, true)
		}
		return toolResponse(result, false)
	case "write_serial":
		result, err := server.service.WriteSerial(ctx, service.SerialWriteConfig{
			ArtifactsRoot: stringArg(params.Arguments, "artifactsRoot", server.artifactsRoot),
			RunDir:        stringArg(params.Arguments, "runDir", ""),
			Data:          stringArg(params.Arguments, "data", ""),
		})
		if err != nil {
			return toolResponse(map[string]any{"error": err.Error()}, true)
		}
		return toolResponse(result, false)
	case "verify_target":
		result, err := server.service.Verify(ctx, service.VerifyConfig{ArtifactsRoot: stringArg(params.Arguments, "artifactsRoot", server.artifactsRoot), RunDir: stringArg(params.Arguments, "runDir", ""), SSHPort: intArg(params.Arguments, "sshPort", 2223), SSHUser: stringArg(params.Arguments, "sshUser", "root"), PrivateKey: stringArg(params.Arguments, "privateKey", filepath.Join(server.artifactsRoot, "cache", "keys", "id_ed25519")), InstalledUser: stringArg(params.Arguments, "installedUser", "skea"), Timeout: time.Duration(intArg(params.Arguments, "timeoutSeconds", 120)) * time.Second})
		if err != nil {
			return toolResponse(map[string]any{"error": err.Error()}, true)
		}
		return toolResponse(result, false)
	default:
		return &response{JSONRPC: "2.0", ID: req.ID, Error: &responseError{Code: -32602, Message: "unknown tool: " + params.Name}}
	}
}

func (server *Server) runAsyncJob(ctx context.Context, jobID string, cfg service.RunConfig) {
	result, err := server.service.RunInstall(ctx, cfg)

	server.mu.Lock()
	defer server.mu.Unlock()
	job := server.jobs[jobID]
	if job == nil {
		return
	}
	job.FinishedAt = time.Now().UTC()
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return
	}
	job.Status = "completed"
	job.Result = result
	job.Error = ""
}

func (server *Server) newJob(kind string) *Job {
	server.mu.Lock()
	defer server.mu.Unlock()
	job := &Job{ID: time.Now().UTC().Format("20060102T150405.000000000"), Kind: kind, Status: "running", StartedAt: time.Now().UTC()}
	server.jobs[job.ID] = job
	return job
}

func (server *Server) jobSnapshot() []*Job {
	server.mu.Lock()
	defer server.mu.Unlock()
	jobs := make([]*Job, 0, len(server.jobs))
	for _, job := range server.jobs {
		clone := *job
		jobs = append(jobs, &clone)
	}
	return jobs
}

func (server *Server) toolDefinitions() []map[string]any {
	return []map[string]any{
		{"name": "prepare_runner", "title": "Prepare Runner Image", "description": "Prepare or refresh the reusable runner image.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"runnerImage": map[string]any{"type": "string"}, "baseImageUrl": map[string]any{"type": "string"}, "aptProxy": map[string]any{"type": "string"}, "enableKvm": map[string]any{"type": "boolean"}}}},
		{"name": "run_install_async", "title": "Run Install Async", "description": "Run the full installer flow asynchronously.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"runnerImage": map[string]any{"type": "string"}, "alpineMirror": map[string]any{"type": "string"}, "keepTargetRunning": map[string]any{"type": "boolean"}, "enableKvm": map[string]any{"type": "boolean"}, "enableTpm": map[string]any{"type": "boolean", "description": "Enable TPM-backed Clevis LUKS auto-unlock. Starts a per-run swtpm emulator, installs Clevis in the Alpine guest, and binds both LUKS devices to the TPM after first boot."}}}},
		{"name": "get_status", "title": "Get Status", "description": "Return active harness state and background MCP jobs.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
		{"name": "stop_active_run", "title": "Stop Active Run", "description": "Stop the currently active VM via QMP.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"force": map[string]any{"type": "boolean"}}}},
		{"name": "cleanup_stale", "title": "Cleanup Stale Harness State", "description": "Kill stale harness-owned QEMU instances and clear stale active-run state when no live run remains.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"force": map[string]any{"type": "boolean"}}}},
		{"name": "tail_log", "title": "Tail Log", "description": "Tail a named harness log file.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"runDir": map[string]any{"type": "string"}, "log": map[string]any{"type": "string"}, "lines": map[string]any{"type": "integer"}}, "required": []string{"log"}}},
		{"name": "write_serial", "title": "Write Target Serial", "description": "Write raw data to the target serial socket for the active run or a specified run.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"runDir": map[string]any{"type": "string"}, "data": map[string]any{"type": "string", "description": "Raw bytes to write to the target serial socket. Include any newline you need."}}, "required": []string{"data"}}},
		{"name": "verify_target", "title": "Verify Target", "description": "Run verification checks against a reachable installed target.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"sshPort": map[string]any{"type": "integer"}, "sshUser": map[string]any{"type": "string"}, "privateKey": map[string]any{"type": "string"}, "installedUser": map[string]any{"type": "string"}}}},
	}
}

func stringArg(args map[string]interface{}, key, fallback string) string {
	if args == nil {
		return fallback
	}
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	stringValue, ok := value.(string)
	if !ok || stringValue == "" {
		return fallback
	}
	return stringValue
}

func boolArg(args map[string]interface{}, key string, fallback bool) bool {
	if args == nil {
		return fallback
	}
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	booleanValue, ok := value.(bool)
	if !ok {
		return fallback
	}
	return booleanValue
}

func intArg(args map[string]interface{}, key string, fallback int) int {
	if args == nil {
		return fallback
	}
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return fallback
	}
}
