// Package app provides application lifecycle management.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nilparra-dev/opencode-go-cc/internal/config"
	"github.com/nilparra-dev/opencode-go-cc/internal/proxy"
)

// App manages the proxy server lifecycle.
type App struct {
	server     *proxy.Server
	atomicCfg  *config.AtomicConfig
	pidFile    string
}

// NewApp creates a new application instance.
func NewApp(cfg *config.Config, pidFile string) (*App, error) {
	atomicCfg := config.NewAtomicConfig(cfg, config.ResolveConfigPath())

	server, err := proxy.NewServer(atomicCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	return &App{
		server:    server,
		atomicCfg: atomicCfg,
		pidFile:   pidFile,
	}, nil
}

// Start runs the application until interrupted.
func (a *App) Start() error {
	// Write PID file
	if a.pidFile != "" {
		if err := os.WriteFile(a.pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
			return fmt.Errorf("failed to write PID file: %w", err)
		}
		defer os.Remove(a.pidFile)
	}

	// Handle shutdown signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		slog.Info("shutting down gracefully")
		cancel()
	}()

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- a.server.Start()
	}()

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return a.server.Stop(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

// Stop gracefully stops the application.
func (a *App) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.server.Stop(ctx)
}
