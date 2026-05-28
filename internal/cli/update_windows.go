//go:build windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func applyReleaseUpdate(stagePath, targetPath string) error {
	scriptPath := filepath.Join(os.TempDir(), "occb-update.ps1")
	backupPath := targetPath + ".old"
	script := fmt.Sprintf(`$ErrorActionPreference = "SilentlyContinue"
$target = %q
$stage = %q
$backup = %q
for ($i = 0; $i -lt 40; $i++) {
    try {
        if (Test-Path $backup) {
            Remove-Item $backup -Force
        }
        if (Test-Path $target) {
            Move-Item -Path $target -Destination $backup -Force
        }
        Move-Item -Path $stage -Destination $target -Force
        if (Test-Path $backup) {
            Remove-Item $backup -Force
        }
        exit 0
    } catch {
        Start-Sleep -Milliseconds 250
    }
}
exit 1
`, targetPath, stagePath, backupPath)

	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		return fmt.Errorf("failed to write update helper: %w", err)
	}

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start update helper: %w", err)
	}

	return nil
}