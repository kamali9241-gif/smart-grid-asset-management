package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/sivakumarkam/smart-grid/backend/internal/repository"
	"github.com/sivakumarkam/smart-grid/backend/internal/service"
)

type Server struct {
	repo           *repository.Repository
	imports        *service.ImportService
	logger         *slog.Logger
	maxUploadBytes int64
}

func NewServer(repo *repository.Repository, imports *service.ImportService, logger *slog.Logger, maxUploadBytes int64) *Server {
	return &Server{repo: repo, imports: imports, logger: logger, maxUploadBytes: maxUploadBytes}
}

func (s *Server) Routes(allowedOrigins []string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type", "X-Request-Id"},
		MaxAge:         300,
	}))

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		r.Get("/meta", s.handleMeta)
		r.Post("/imports", s.handleImport)
		r.Route("/assets", func(r chi.Router) {
			r.Get("/roots", s.handleRoots)
			r.Get("/search", s.handleSearch)
			r.Get("/{assetId}", s.handleAsset)
			r.Get("/{assetId}/children", s.handleChildren)
			r.Get("/{assetId}/ancestors", s.handleAncestors)
		})
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "unknown endpoint "+r.URL.Path)
	})
	return r
}

// requestLogger emits one structured line per request.
func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()))
	})
}
