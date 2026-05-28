//go:build !windows

package cli

import (
	"fmt"
	"os"
)

func applyReleaseUpdate(stagePath, targetPath string) error {
	if err := os.Chmod(stagePath, 0755); err != nil {
		return fmt.Errorf("failed to mark update as executable: %w", err)
	}

	if err := os.Rename(stagePath, targetPath); err != nil {
		return fmt.Errorf("failed to replace executable: %w", err)
	}

	return nil
}