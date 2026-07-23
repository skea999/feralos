package runstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ActiveRun struct {
	RunID            string    `json:"runId"`
	Mode             string    `json:"mode"`
	Status           string    `json:"status"`
	Phase            string    `json:"phase"`
	RunDir           string    `json:"runDir"`
	RunnerQMPPath    string    `json:"runnerQmpPath,omitempty"`
	TargetQMPPath    string    `json:"targetQmpPath,omitempty"`
	RunnerSerialLog  string    `json:"runnerSerialLog,omitempty"`
	TargetSerialLog  string    `json:"targetSerialLog,omitempty"`
	RunnerQemuLog    string    `json:"runnerQemuLog,omitempty"`
	TargetQemuLog    string    `json:"targetQemuLog,omitempty"`
	TargetFirmwareLog string   `json:"targetFirmwareLog,omitempty"`
	InstallerLogPath string    `json:"installerLogPath,omitempty"`
	VerifyLogPath    string    `json:"verifyLogPath,omitempty"`
	RunnerSSHPort    int       `json:"runnerSshPort,omitempty"`
	TargetSSHPort    int       `json:"targetSshPort,omitempty"`
	UpdatedAt        time.Time `json:"updatedAt"`
	StartedAt        time.Time `json:"startedAt"`
	Error            string    `json:"error,omitempty"`
}

type Store struct {
	root string
}

func New(root string) Store {
	return Store{root: root}
}

func (store Store) Save(active ActiveRun) error {
	active.UpdatedAt = time.Now().UTC()
	if active.StartedAt.IsZero() {
		active.StartedAt = active.UpdatedAt
	}

	if err := os.MkdirAll(store.stateDir(), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	blob, err := json.MarshalIndent(active, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal active state: %w", err)
	}

	return os.WriteFile(store.activePath(), blob, 0o644)
}

func (store Store) Load() (ActiveRun, error) {
	blob, err := os.ReadFile(store.activePath())
	if err != nil {
		return ActiveRun{}, fmt.Errorf("read active state: %w", err)
	}

	var active ActiveRun
	if err := json.Unmarshal(blob, &active); err != nil {
		return ActiveRun{}, fmt.Errorf("unmarshal active state: %w", err)
	}

	return active, nil
}

func (store Store) Clear() error {
	if err := os.Remove(store.activePath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove active state: %w", err)
	}

	return nil
}

func (store Store) ActivePath() string {
	return store.activePath()
}

func (store Store) stateDir() string {
	return filepath.Join(store.root, "state")
}

func (store Store) activePath() string {
	return filepath.Join(store.stateDir(), "active.json")
}
