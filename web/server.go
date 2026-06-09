// web/server.go
package web

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/Shadow-Azure/Golem/internal/core"
	"github.com/Shadow-Azure/Golem/internal/plugin"
)

// Server represents the WebChat HTTP server.
type Server struct {
	engine   *core.Engine
	provider plugin.ProviderPlugin
	session  *core.Session
	addr     string
	logger   *slog.Logger
	mu       sync.RWMutex
}

// NewServer creates a new WebChat server.
func NewServer(engine *core.Engine, provider plugin.ProviderPlugin, addr string) *Server {
	return &Server{
		engine:   engine,
		provider: provider,
		addr:     addr,
		logger:   slog.Default().With("component", "webchat"),
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Static files - look relative to binary location
	execPath, err := os.Executable()
	if err != nil {
		s.logger.Warn("failed to get executable path, using current dir", "error", err)
		execPath = "."
	}
	execDir := filepath.Dir(execPath)
	staticDir := filepath.Join(execDir, "web", "static")

	if _, err := os.Stat(staticDir); err == nil {
		mux.Handle("/", http.FileServer(http.Dir(staticDir)))
		s.logger.Info("serving static files", "dir", staticDir)
	} else {
		s.logger.Warn("static files not found", "dir", staticDir)
	}

	// API endpoints
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/clear", s.handleClear)
	mux.HandleFunc("/api/stream", s.handleStream)

	s.logger.Info("starting webchat server", "addr", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

// GetAddr returns the server address.
func (s *Server) GetAddr() string {
	return s.addr
}
