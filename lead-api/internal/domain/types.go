package domain

import "time"

// SearchRequest é o corpo da requisição POST /api/v1/search
type SearchRequest struct {
	Query           string `json:"query"`
	Location        string `json:"location"`
	EnrichCNPJ      bool   `json:"enrich_cnpj"`
	EnrichInstagram bool   `json:"enrich_instagram"`
}

// Lead é o lead enriquecido retornado pela API
type Lead struct {
	Name      string   `json:"name"`
	Phone     string   `json:"phone,omitempty"`
	CNPJ      string   `json:"cnpj,omitempty"`
	Partners  []string `json:"partners,omitempty"`
	Instagram string   `json:"instagram,omitempty"`
	Followers string   `json:"followers,omitempty"`
	CNAEMatch *bool    `json:"cnae_match,omitempty"`
	Municipio string   `json:"municipio,omitempty"`
	UF        string   `json:"uf,omitempty"`
	Source    string   `json:"source,omitempty"`
}

// SearchResponse é a resposta da API
type SearchResponse struct {
	Query      string    `json:"query"`
	Location   string    `json:"location"`
	Total      int       `json:"total"`
	Discarded  int       `json:"discarded,omitempty"` // leads filtrados por cidade/CNAE
	Cached     bool      `json:"cached"`
	SearchID   string    `json:"search_id,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	DurationMs int64     `json:"duration_ms"`
	Leads      []Lead    `json:"leads"`
}

// StoredSearch é o documento salvo no MongoDB
type StoredSearch struct {
	ID              string    `bson:"_id,omitempty"   json:"id"`
	Query           string    `bson:"query"           json:"query"`
	Location        string    `bson:"location"        json:"location"`
	EnrichCNPJ      bool      `bson:"enrich_cnpj"     json:"enrich_cnpj"`
	EnrichInstagram bool      `bson:"enrich_instagram" json:"enrich_instagram"`
	Total           int       `bson:"total"           json:"total"`
	DurationMs      int64     `bson:"duration_ms"     json:"duration_ms"`
	Leads           []Lead    `bson:"leads"           json:"leads"`
	CreatedAt       time.Time `bson:"created_at"      json:"created_at"`
	ExpiresAt       time.Time `bson:"expires_at"      json:"expires_at"`
}

// CachedEnrichment é o cache por lead individual no MongoDB
type CachedEnrichment struct {
	Key       string    `bson:"_id"` // sha256(name+city)
	CNPJ      string    `bson:"cnpj"`
	Partners  []string  `bson:"partners"`
	CNAECode  string    `bson:"cnae_code"`
	CNAEDesc  string    `bson:"cnae_desc"`
	Municipio string    `bson:"municipio"`
	UF        string    `bson:"uf"`
	Instagram string    `bson:"instagram"`
	Followers string    `bson:"followers"`
	UpdatedAt time.Time `bson:"updated_at"`
	ExpiresAt time.Time `bson:"expires_at"`
}
