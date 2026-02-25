package api

import (
	"context"
	"log"
	"net/http"
	"time"
)

// Server wraps the HTTP server.
type Server struct {
	srv *http.Server
}

// NewServer wires routes and returns a ready-to-start Server.
func NewServer(addr string, h *Handler) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/api/v1/search", h.Search)
	mux.HandleFunc("/api/v1/search/cache", h.InvalidateCache)

	return &Server{
		srv: &http.Server{
			Addr:         addr,
			Handler:      loggingMiddleware(mux),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 5 * time.Minute, // searches can be slow
			IdleTimeout:  60 * time.Second,
		},
	}
}

// Start begins listening and blocks until the server stops.
func (s *Server) Start() error {
	log.Printf("lead-api listening on %s", s.srv.Addr)
	return s.srv.ListenAndServe()
}

// Shutdown gracefully shuts down with the given context.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// loggingMiddleware logs each request with method, path and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}
