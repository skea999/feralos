package utility

import (
	"os"
)

func WriteFileInChroot(chroot string, path string, content string) error {
	fullPath := chroot + path
	return os.WriteFile(fullPath, []byte(content), 0644)
}
