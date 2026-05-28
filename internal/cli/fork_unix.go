//go:build !windows

// Package cli provides cross-platform background process spawning for Unix.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// forkIntoBackground re-executes the current process in the background.
func forkIntoBackground(port int) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, "serve", "--_daemonize")
	if port != 0 {
		cmd.Args = append(cmd.Args, "--port", fmt.Sprintf("%d", port))
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	return cmd.Start()
}
