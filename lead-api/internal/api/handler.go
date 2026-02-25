package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/lucasfdcampos/lead-api/internal/cache"
	"github.com/lucasfdcampos/lead-api/internal/domain"
	"github.com/lucasfdcampos/lead-api/internal/pipeline"
	"github.com/lucasfdcampos/lead-api/internal/store"
)

// Handler holds the HTTP dependencies.
type Handler struct {
	redis *cache.Client
	mongo *store.Client
}

// NewHandler creates a new Handler.
func NewHandler(redis *cache.Client, mongo *store.Client) *Handler {
	return &Handler{redis: redis, mongo: mongo}
}

// errResponse writes a JSON error body.
func errResponse(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Health godoc
//
//	GET /health
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
}

// Search godoc
//
//	POST /api/v1/search
//
//	Request body: { "query": "...", "location": "...", "enrich_cnpj": true, "enrich_instagram": false }
//	Response:     SearchResponse JSON
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req domain.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errResponse(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.Query == "" {
		errResponse(w, http.StatusBadRequest, "query is required")
		return
	}
	if req.Location == "" {
		errResponse(w, http.StatusBadRequest, "location is required")
		return
	}

	cfg := pipeline.Config{
		Redis: h.redis,
		Mongo: h.mongo,
	}

	resp, err := pipeline.Run(r.Context(), req, cfg)
	if err != nil {
		errResponse(w, http.StatusInternalServerError, "pipeline error: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// InvalidateCache godoc
//
//	DELETE /api/v1/search/cache
//
//	Query params: query, location, enrich_cnpj (0|1), enrich_instagram (0|1)
func (h *Handler) InvalidateCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.redis == nil {
		errResponse(w, http.StatusServiceUnavailable, "redis not configured")
		return
	}

	q := r.URL.Query()
	query := q.Get("query")
	location := q.Get("location")
	if query == "" || location == "" {
		errResponse(w, http.StatusBadRequest, "query and location are required")
		return
	}

	ec := q.Get("enrich_cnpj") == "1"
	ei := q.Get("enrich_instagram") == "1"
	key := cache.SearchKey(query, location, ec, ei)

	if err := h.redis.DeleteSearch(r.Context(), key); err != nil {
		errResponse(w, http.StatusInternalServerError, "failed to delete cache key: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "key": key})
}
