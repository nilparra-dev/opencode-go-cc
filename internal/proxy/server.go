// Package proxy implements the HTTP proxy server.
package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nilparra-dev/opencode-go-cc/internal/client"
	"github.com/nilparra-dev/opencode-go-cc/internal/config"
	"github.com/nilparra-dev/opencode-go-cc/internal/router"
	"github.com/nilparra-dev/opencode-go-cc/internal/token"
	"github.com/nilparra-dev/opencode-go-cc/internal/transform"
)

// Server is the HTTP proxy server.
type Server struct {
	cfg           *config.AtomicConfig
	httpServer    *http.Server
	router        *router.ModelSelector
	reqTransformer  *transform.RequestTransformer
	respTransformer *transform.ResponseTransformer
	ocClient      *client.OpenCodeClient
	tokenCounter  *token.Counter
}

// NewServer creates a new proxy server.
func NewServer(atomicCfg *config.AtomicConfig) (*Server, error) {
	cfg := atomicCfg.Get()

	ocClient := client.NewOpenCodeClient(&cfg.OpenCodeGo, cfg.APIKey)
	tokenCounter, err := token.NewCounter()
	if err != nil {
		slog.Warn("failed to create token counter, using fallback", "error", err)
		tokenCounter = nil
	}

	s := &Server{
		cfg:             atomicCfg,
		router:          router.NewModelSelector(atomicCfg),
		reqTransformer:  transform.NewRequestTransformer(),
		respTransformer: transform.NewResponseTransformer(),
		ocClient:        ocClient,
		tokenCounter:    tokenCounter,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/messages", s.handleMessages)
	mux.HandleFunc("/v1/messages/count_tokens", s.handleCountTokens)
	mux.HandleFunc("/v1/models", s.handleModels)

	handler := withLogging(withRecovery(mux))

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: handler,
	}

	return s, nil
}

// Start begins listening for requests.
func (s *Server) Start() error {
	slog.Info("starting proxy server", "addr", s.httpServer.Addr)

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(ctx)
	}()

	return s.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
