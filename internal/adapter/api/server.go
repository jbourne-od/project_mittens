package api

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/optimaldynamics/project-mittens/internal/adapter/db"
	"github.com/optimaldynamics/project-mittens/internal/service"
	pkgjournal "github.com/optimaldynamics/project-mittens/pkg/journal"
	"github.com/optimaldynamics/project-mittens/pkg/telemetry"
	"github.com/optimaldynamics/project-mittens/web"
)

// ServerConfig configures the HTTP REST API server.
type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeoutSec  int
	WriteTimeoutSec int
	IdleTimeoutSec  int
	DatabaseURL     string
	DBConfig        *db.DBConfig
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

	var (
		pool    *db.Pool
		jStore  service.Journal
		cStore  pkgjournal.JournalStore
		runRepo *db.PostgresRunRepository
	)

	dbConnStr := cfg.DatabaseURL
	if dbConnStr == "" && cfg.DBConfig != nil {
		dbConnStr = cfg.DBConfig.ConnString()
	}

	if dbConnStr != "" {
		dbCfg, err := db.ParseURL(dbConnStr)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			p, err := db.NewPool(ctx, dbCfg)
			cancel()
			if err == nil {
				pool = p
				pgStore := db.NewPostgresJournalStore(pool)
				jStore = pgStore
				cStore = pgStore
				runRepo = db.NewPostgresRunRepository(pool)
			}
		}
	}

	h := NewHandlerWithDeps(HandlerDependencies{
		Journal:       jStore,
		CryptoStore:   cStore,
		DBPool:        pool,
		RunRepository: runRepo,
	})

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
	r.Handle("/metrics", telemetry.GlobalProvider().PrometheusHandler())

	// API v1 Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/optimize", h.HandleOptimize)
		r.Post("/simulate", h.HandleSimulate)
		r.Get("/scenarios", h.HandleListScenarios)
		r.Get("/scenarios/{id}", h.HandleGetScenario)
		r.Get("/decisions", h.HandleListDecisions)
		r.Get("/decisions/{id}", h.HandleGetDecision)
		r.Get("/decisions/{id}/explain", h.HandleExplainDecision)
		r.Post("/decisions/{id}/replay", h.HandleReplayDecision)
		r.Get("/runs/{id}/integrity", h.HandleVerifyRunIntegrity)

		// Streaming Ingestion
		r.Post("/stream/telemetry", h.HandleStreamTelemetry)
		r.Post("/stream/tenders", h.HandleStreamTenders)
		r.Post("/stream/cancels", h.HandleStreamCancels)
		r.Get("/stream/status", h.HandleStreamStatus)

		// Fleet Repositioning
		r.Post("/reposition/plan", h.HandleRepositionPlan)
	})

	// Static Web Frontend (Embedded SPA with filesystem override)
	var staticFS fs.FS
	if customDir := os.Getenv("MITTENS_STATIC_DIR"); customDir != "" {
		if stat, err := os.Stat(customDir); err == nil && stat.IsDir() {
			staticFS = os.DirFS(customDir)
		}
	}
	if staticFS == nil {
		if sub, err := fs.Sub(web.Assets, "dist"); err == nil {
			staticFS = sub
		}
	}

	if staticFS != nil {
		fileServer := http.FileServer(http.FS(staticFS))
		r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
			if strings.HasPrefix(req.URL.Path, "/api") || strings.HasPrefix(req.URL.Path, "/metrics") || strings.HasPrefix(req.URL.Path, "/healthz") {
				http.NotFound(w, req)
				return
			}

			path := strings.TrimPrefix(req.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}

			f, err := staticFS.Open(path)
			if err != nil {
				// SPA Client-side Route Fallback -> serve index.html
				indexF, err := staticFS.Open("index.html")
				if err != nil {
					http.NotFound(w, req)
					return
				}
				defer indexF.Close()
				stat, _ := indexF.Stat()
				http.ServeContent(w, req, "index.html", stat.ModTime(), indexF.(io.ReadSeeker))
				return
			}
			defer f.Close()

			fileServer.ServeHTTP(w, req)
		})
	}

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

// Shutdown gracefully stops the HTTP server and closes database connection pools.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.handler != nil && s.handler.dbPool != nil {
		s.handler.dbPool.Close()
	}
	return s.httpSrv.Shutdown(ctx)
}
