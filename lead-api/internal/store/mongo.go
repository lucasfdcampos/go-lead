// Package store provides MongoDB persistence for searches and enrichment data.
//
// Collections (all in database "lead_api"):
//   - searches     – search metadata, no embedded leads (TTL: 30 days)
//   - results      – individual lead results linked to a search via search_id (TTL: 30 days)
//   - enrichments  – per-lead CNPJ/Instagram data (TTL: 30 days)
//   - cnae_hints   – CNAE codes discovered dynamically for a query (TTL: 90 days)
//   - cnaes        – CNAE reference data (static, managed externally)
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
	dbName            = "lead_api"
	searchCollection  = "searches"
	resultsCollection = "results"
	enrichCollection  = "enrichments"
	cnaeHintsCol      = "cnae_hints"
	cnaesCol          = "cnaes"

	searchTTLDays   = 30
	enrichTTLDays   = 30
	cnaeHintTTLDays = 90
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
	// searches: TTL + lookup on (query, location, flags)
	sc := c.mdb.Collection(searchCollection)
	if _, err := sc.Indexes().CreateMany(ctx, []mongo.IndexModel{
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
	}); err != nil {
		return fmt.Errorf("store: search indices: %w", err)
	}

	// results: TTL + lookup on search_id
	rc := c.mdb.Collection(resultsCollection)
	if _, err := rc.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
		{
			Keys: bson.D{{Key: "search_id", Value: 1}},
		},
	}); err != nil {
		return fmt.Errorf("store: results indices: %w", err)
	}

	// enrichments: TTL only (_id is indexed by default)
	ec := c.mdb.Collection(enrichCollection)
	if _, err := ec.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	}); err != nil {
		return fmt.Errorf("store: enrichment indices: %w", err)
	}

	// cnae_hints: TTL on updated_at (_id = query string)
	hc := c.mdb.Collection(cnaeHintsCol)
	if _, err := hc.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "updated_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(int32(cnaeHintTTLDays * 24 * 3600)),
	}); err != nil {
		return fmt.Errorf("store: cnae_hints indices: %w", err)
	}

	return nil
}

// ─── Searches ─────────────────────────────────────────────────────────────────

// SaveSearch persists search metadata (without embedded leads).
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

// ─── Results ──────────────────────────────────────────────────────────────────

// SaveResults inserts individual lead results linked to a searchID.
func (c *Client) SaveResults(ctx context.Context, searchID string, leads []domain.Lead) error {
	if len(leads) == 0 {
		return nil
	}
	now := time.Now().UTC()
	exp := now.Add(searchTTLDays * 24 * time.Hour)

	docs := make([]any, 0, len(leads))
	for _, l := range leads {
		docs = append(docs, domain.StoredResult{
			SearchID:  searchID,
			Lead:      l,
			CreatedAt: now,
			ExpiresAt: exp,
		})
	}

	_, err := c.mdb.Collection(resultsCollection).InsertMany(ctx, docs)
	if err != nil {
		return fmt.Errorf("store: save results: %w", err)
	}
	return nil
}

// FindResultsBySearchID retrieves all leads for the given searchID, in insertion order.
func (c *Client) FindResultsBySearchID(ctx context.Context, searchID string) ([]domain.Lead, error) {
	cursor, err := c.mdb.Collection(resultsCollection).Find(ctx,
		bson.M{"search_id": searchID},
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("store: find results: %w", err)
	}
	defer cursor.Close(ctx)

	var leads []domain.Lead
	for cursor.Next(ctx) {
		var doc domain.StoredResult
		if err := cursor.Decode(&doc); err == nil {
			leads = append(leads, doc.Lead)
		}
	}
	return leads, cursor.Err()
}

// ─── CNAE Hints ───────────────────────────────────────────────────────────────

// GetCNAEHint returns a cached CNAE hint for the given query, or nil if not found.
func (c *Client) GetCNAEHint(ctx context.Context, query string) (*domain.CNAEHintDoc, error) {
	var doc domain.CNAEHintDoc
	err := c.mdb.Collection(cnaeHintsCol).FindOne(ctx, bson.M{"_id": query}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get cnae hint: %w", err)
	}
	return &doc, nil
}

// SaveCNAEHint upserts a CNAE hint for the given query.
func (c *Client) SaveCNAEHint(ctx context.Context, hint *domain.CNAEHintDoc) error {
	hint.UpdatedAt = time.Now().UTC()
	filter := bson.M{"_id": hint.Query}
	update := bson.M{"$set": hint}
	opts := options.Update().SetUpsert(true)
	_, err := c.mdb.Collection(cnaeHintsCol).UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("store: save cnae hint: %w", err)
	}
	return nil
}

// ─── CNAE reference (lead_api.cnaes) ──────────────────────────────────────────

// QueryCNAEs returns CNAE codes from the local lead_api.cnaes collection
// whose description contains any of the given keywords.
func (c *Client) QueryCNAEs(ctx context.Context, keywords []string) ([]string, error) {
	if len(keywords) == 0 {
		return nil, nil
	}

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

	cursor, err := c.mdb.Collection(cnaesCol).Find(ctx, bson.M{"$or": ors},
		options.Find().SetProjection(bson.M{"codigo": 1}))
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

// QueryLeadfinderCNAEs is kept for backward compatibility; delegates to QueryCNAEs.
func (c *Client) QueryLeadfinderCNAEs(ctx context.Context, keywords []string) ([]string, error) {
	return c.QueryCNAEs(ctx, keywords)
}

// ─── Enrichment cache ─────────────────────────────────────────────────────────

// CachedEnrichment is the MongoDB document for per-lead enrichment.
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
