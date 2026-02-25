// Package store provides MongoDB persistence for searches and enrichment data.
//
// Collections:
//   - searches      – full search results (TTL index: 30 days)
//   - enrichments   – per-lead CNPJ/Instagram data (TTL index: 30 days)
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/lucasfdcampos/lead-api/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	dbName           = "lead_api"
	searchCollection = "searches"
	enrichCollection = "enrichments"
	searchTTLDays    = 30
	enrichTTLDays    = 30
)

// Client wraps a MongoDB client.
type Client struct {
	mc  *mongo.Client
	mdb *mongo.Database
}

// New connects to MongoDB and returns a store Client.
func New(ctx context.Context, uri string) (*Client, error) {
	clientOpts := options.Client().ApplyURI(uri)
	mc, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("store: mongo connect: %w", err)
	}
	if err := mc.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("store: mongo ping: %w", err)
	}

	db := mc.Database(dbName)
	c := &Client{mc: mc, mdb: db}

	// Ensure TTL indices
	if err := c.ensureIndices(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

// Disconnect cleanly closes the MongoDB connection.
func (c *Client) Disconnect(ctx context.Context) error {
	return c.mc.Disconnect(ctx)
}

// MongoClient exposes the underlying *mongo.Client for cross-database queries.
func (c *Client) MongoClient() *mongo.Client {
	return c.mc
}

// ensureIndices creates TTL and lookup indices if missing.
func (c *Client) ensureIndices(ctx context.Context) error {
	// searches: TTL on expires_at + lookup on (query, location)
	sc := c.mdb.Collection(searchCollection)
	_, err := sc.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
		{
			Keys: bson.D{
				{Key: "query", Value: 1},
				{Key: "location", Value: 1},
				{Key: "enrich_cnpj", Value: 1},
				{Key: "enrich_instagram", Value: 1},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("store: search indices: %w", err)
	}

	// enrichments: TTL + lookup on _id (already indexed)
	ec := c.mdb.Collection(enrichCollection)
	_, err = ec.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	})
	if err != nil {
		return fmt.Errorf("store: enrichment indices: %w", err)
	}

	return nil
}

// ─── Searches ─────────────────────────────────────────────────────────────────

// SaveSearch persists a completed search result.
func (c *Client) SaveSearch(ctx context.Context, s *domain.StoredSearch) (string, error) {
	s.CreatedAt = time.Now().UTC()
	s.ExpiresAt = s.CreatedAt.Add(searchTTLDays * 24 * time.Hour)

	res, err := c.mdb.Collection(searchCollection).InsertOne(ctx, s)
	if err != nil {
		return "", fmt.Errorf("store: save search: %w", err)
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		return oid.Hex(), nil
	}
	return "", nil
}

// FindSearch looks up a recent search by query/location/flags.
// Returns nil, nil when not found.
func (c *Client) FindSearch(ctx context.Context, query, location string, enrichCNPJ, enrichInstagram bool) (*domain.StoredSearch, error) {
	filter := bson.M{
		"query":            query,
		"location":         location,
		"enrich_cnpj":      enrichCNPJ,
		"enrich_instagram": enrichInstagram,
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})

	var s domain.StoredSearch
	err := c.mdb.Collection(searchCollection).FindOne(ctx, filter, opts).Decode(&s)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: find search: %w", err)
	}
	return &s, nil
}

// ─── Enrichment cache ─────────────────────────────────────────────────────────

// CachedEnrichment mirrors the MongoDB document for per-lead enrichment.
type CachedEnrichment struct {
	Key       string    `bson:"_id"`
	CNPJ      string    `bson:"cnpj,omitempty"`
	Partners  []string  `bson:"partners,omitempty"`
	CNAECode  string    `bson:"cnae_code,omitempty"`
	CNAEDesc  string    `bson:"cnae_desc,omitempty"`
	Municipio string    `bson:"municipio,omitempty"`
	UF        string    `bson:"uf,omitempty"`
	Instagram string    `bson:"instagram,omitempty"`
	Followers string    `bson:"followers,omitempty"`
	UpdatedAt time.Time `bson:"updated_at"`
	ExpiresAt time.Time `bson:"expires_at"`
}

// QueryLeadfinderCNAEs returns CNAE codes from the leadfinder database whose
// description contains any of the given keywords (case-insensitive).
// Returns an empty slice (not an error) when the collection is not reachable.
func (c *Client) QueryLeadfinderCNAEs(ctx context.Context, keywords []string) ([]string, error) {
	if len(keywords) == 0 {
		return nil, nil
	}

	// Build $or filter: each keyword as a case-insensitive regex on descricao
	ors := make(bson.A, 0, len(keywords))
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		ors = append(ors, bson.M{"descricao": bson.M{"$regex": kw, "$options": "i"}})
	}
	if len(ors) == 0 {
		return nil, nil
	}

	coll := c.mc.Database("leadfinder").Collection("cnaes")
	cursor, err := coll.Find(ctx, bson.M{"$or": ors}, options.Find().SetProjection(bson.M{"codigo": 1}))
	if err != nil {
		return nil, fmt.Errorf("store: query cnaes: %w", err)
	}
	defer cursor.Close(ctx)

	type cnaedoc struct {
		Codigo string `bson:"codigo"`
	}
	var codes []string
	for cursor.Next(ctx) {
		var doc cnaedoc
		if err := cursor.Decode(&doc); err == nil && doc.Codigo != "" {
			codes = append(codes, doc.Codigo)
		}
	}
	return codes, cursor.Err()
}

// GetEnrichment returns cached per-lead enrichment data or nil.
func (c *Client) GetEnrichment(ctx context.Context, key string) (*CachedEnrichment, error) {
	var e CachedEnrichment
	err := c.mdb.Collection(enrichCollection).FindOne(ctx, bson.M{"_id": key}).Decode(&e)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get enrichment: %w", err)
	}
	return &e, nil
}

// SaveEnrichment upserts enrichment data for a lead.
func (c *Client) SaveEnrichment(ctx context.Context, e *CachedEnrichment) error {
	e.UpdatedAt = time.Now().UTC()
	e.ExpiresAt = e.UpdatedAt.Add(enrichTTLDays * 24 * time.Hour)

	filter := bson.M{"_id": e.Key}
	update := bson.M{"$set": e}
	opts := options.Update().SetUpsert(true)

	_, err := c.mdb.Collection(enrichCollection).UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("store: save enrichment: %w", err)
	}
	return nil
}
