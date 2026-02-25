// Package cache provides a Redis-backed caching layer.
//
// Key strategy:
//   - Search results:       lead:search:v1:{sha256(query+location+flags)} → TTL 24 h
//   - Per-lead enrichment:  lead:enrich:v1:{sha256(name+city)}            → TTL 7 d
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

const (
	SearchTTL     = 24 * time.Hour
	EnrichmentTTL = 7 * 24 * time.Hour

	searchPrefix     = "lead:search:v1:"
	enrichmentPrefix = "lead:enrich:v1:"
)

// Client wraps redis.Client with domain-aware helpers.
type Client struct {
	rdb *redis.Client
}

// New creates a new cache Client.
// addr example: "localhost:6379"
func New(addr, password string, db int) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Client{rdb: rdb}
}

// Ping checks connectivity.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Close closes the Redis connection.
func (c *Client) Close() error { return c.rdb.Close() }

// ─── Search cache ──────────────────────────────────────────────────────────────

// SearchKey returns the cache key for a search.
func SearchKey(query, location string, enrichCNPJ, enrichInstagram bool) string {
	raw := fmt.Sprintf("%s|%s|cnpj=%v|ig=%v", query, location, enrichCNPJ, enrichInstagram)
	h := sha256.Sum256([]byte(raw))
	return searchPrefix + fmt.Sprintf("%x", h)
}

// GetSearch returns a cached value (as raw JSON bytes) or nil on miss.
func (c *Client) GetSearch(ctx context.Context, key string) ([]byte, error) {
	val, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // cache miss
	}
	return val, err
}

// SetSearch stores a value with SearchTTL.
func (c *Client) SetSearch(ctx context.Context, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, b, SearchTTL).Err()
}

// DeleteSearch removes a search cache entry.
func (c *Client) DeleteSearch(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// ─── Enrichment cache ─────────────────────────────────────────────────────────

// EnrichedLead is the structure stored per lead in cache.
type EnrichedLead struct {
	CNPJ      string   `json:"cnpj,omitempty"`
	Partners  []string `json:"partners,omitempty"`
	CNAECode  string   `json:"cnae_code,omitempty"`
	CNAEDesc  string   `json:"cnae_desc,omitempty"`
	Municipio string   `json:"municipio,omitempty"`
	UF        string   `json:"uf,omitempty"`
	Instagram string   `json:"instagram,omitempty"`
	Followers string   `json:"followers,omitempty"`
}

// EnrichmentKey returns cache key for per-lead enrichment data.
func EnrichmentKey(name, city string) string {
	raw := name + "|" + city
	h := sha256.Sum256([]byte(raw))
	return enrichmentPrefix + fmt.Sprintf("%x", h)
}

// GetEnrichment returns cached enrichment for a lead, or nil on miss.
func (c *Client) GetEnrichment(ctx context.Context, key string) (*EnrichedLead, error) {
	val, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var e EnrichedLead
	if err := json.Unmarshal(val, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// SetEnrichment stores enrichment data with EnrichmentTTL.
func (c *Client) SetEnrichment(ctx context.Context, key string, e *EnrichedLead) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, b, EnrichmentTTL).Err()
}
