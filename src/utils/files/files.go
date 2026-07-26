package files

import (
	"os"
	"path/filepath"
)

func WriteFileInChroot(chroot string, path string, content string) error {
	fullPath := chroot + path
	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(content), 0644)
}
