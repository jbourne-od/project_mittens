package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// ServerConfig configures the HTTP REST API server.
type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeoutSec  int
	WriteTimeoutSec int
	IdleTimeoutSec  int
}

// DefaultServerConfig returns production-ready server defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:            "0.0.0.0",
		Port:            8080,
		ReadTimeoutSec:  15,
		WriteTimeoutSec: 30,
		IdleTimeoutSec:  60,
	}
}

// Server encapsulates the Chi HTTP multiplexer and lifecycle management.
type Server struct {
	cfg     ServerConfig
	router  chi.Router
	handler *Handler
	httpSrv *http.Server
}

// NewServer initializes Chi routes, middleware, and handlers.
func NewServer(cfg ServerConfig) *Server {
	r := chi.NewRouter()
	h := NewHandler()

	// Middleware Stack
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS Configuration for web UIs / dispatch portals
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// System Routes
	r.Get("/healthz", h.HandleHealth)
	r.Get("/metrics", h.HandleMetrics)

	// API v1 Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/optimize", h.HandleOptimize)
		r.Post("/simulate", h.HandleSimulate)
	})

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeoutSec) * time.Second,
		IdleTimeout:  time.Duration(cfg.IdleTimeoutSec) * time.Second,
	}

	return &Server{
		cfg:     cfg,
		router:  r,
		handler: h,
		httpSrv: srv,
	}
}

// Router returns the underlying Chi router for testing.
func (s *Server) Router() chi.Router {
	return s.router
}

// Start launches the HTTP server synchronously.
func (s *Server) Start() error {
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}
