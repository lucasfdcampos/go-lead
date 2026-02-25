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
	// Dados brutos dos scrapers
	Name     string `json:"name"`
	Phone    string `json:"phone,omitempty"`
	Phone2   string `json:"phone2,omitempty"`
	Address  string `json:"address,omitempty"`
	City     string `json:"city,omitempty"`
	State    string `json:"state,omitempty"`
	Category string `json:"category,omitempty"`
	Website  string `json:"website,omitempty"`
	Email    string `json:"email,omitempty"`
	Source   string `json:"source,omitempty"`

	// Dados do enriquecimento CNPJ
	CNPJ         string   `json:"cnpj,omitempty"`
	RazaoSocial  string   `json:"razao_social,omitempty"`
	NomeFantasia string   `json:"nome_fantasia,omitempty"`
	Situacao     string   `json:"situacao,omitempty"`
	Partners     []string `json:"partners,omitempty"`
	CNAEMatch    *bool    `json:"cnae_match,omitempty"`
	CNAEDesc     string   `json:"cnae_desc,omitempty"`
	Municipio    string   `json:"municipio,omitempty"`
	UF           string   `json:"uf,omitempty"`

	// Dados do enriquecimento Instagram
	Instagram string `json:"instagram,omitempty"`
	Followers string `json:"followers,omitempty"`
}

// SearchResponse é a resposta da API
type SearchResponse struct {
	Query         string    `json:"query"`
	Location      string    `json:"location"`
	Total         int       `json:"total"`
	Discarded     int       `json:"discarded,omitempty"` // leads filtrados por cidade/CNAE
	Cached        bool      `json:"cached"`
	SearchID      string    `json:"search_id,omitempty"`
	CNAEHintCodes []string  `json:"cnae_hint_codes,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	DurationMs    int64     `json:"duration_ms"`
	Leads         []Lead    `json:"leads"`
}

// StoredSearch é o documento de metadados da busca salvo no MongoDB (collection: searches).
// Os leads ficam na collection separada "results", referenciados pelo SearchID.
type StoredSearch struct {
	ID              string    `bson:"_id,omitempty"        json:"id"`
	Query           string    `bson:"query"                json:"query"`
	Location        string    `bson:"location"             json:"location"`
	EnrichCNPJ      bool      `bson:"enrich_cnpj"          json:"enrich_cnpj"`
	EnrichInstagram bool      `bson:"enrich_instagram"     json:"enrich_instagram"`
	Total           int       `bson:"total"                json:"total"`
	Discarded       int       `bson:"discarded"            json:"discarded"`
	DurationMs      int64     `bson:"duration_ms"          json:"duration_ms"`
	CNAEHintCodes   []string  `bson:"cnae_hint_codes,omitempty" json:"cnae_hint_codes,omitempty"`
	CreatedAt       time.Time `bson:"created_at"           json:"created_at"`
	ExpiresAt       time.Time `bson:"expires_at"           json:"expires_at"`
}

// StoredResult é um lead individual vinculado a uma busca (collection: results).
type StoredResult struct {
	ID        string    `bson:"_id,omitempty"  json:"id"`
	SearchID  string    `bson:"search_id"      json:"search_id"`
	Lead      Lead      `bson:"lead"           json:"lead"`
	CreatedAt time.Time `bson:"created_at"     json:"created_at"`
	ExpiresAt time.Time `bson:"expires_at"     json:"expires_at"`
}

// CNAEHintDoc armazena os códigos CNAE descobertos dinamicamente para uma query.
type CNAEHintDoc struct {
	Query     string    `bson:"_id"        json:"query"`
	Codes     []string  `bson:"codes"      json:"codes"`
	Snippet   string    `bson:"snippet"    json:"snippet"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// CachedEnrichment é o cache por lead individual no MongoDB (collection: enrichments)
type CachedEnrichment struct {
	Key          string    `bson:"_id"` // sha256(name+city)
	CNPJ         string    `bson:"cnpj"`
	RazaoSocial  string    `bson:"razao_social"`
	NomeFantasia string    `bson:"nome_fantasia"`
	Situacao     string    `bson:"situacao"`
	Partners     []string  `bson:"partners"`
	CNAECode     string    `bson:"cnae_code"`
	CNAEDesc     string    `bson:"cnae_desc"`
	Municipio    string    `bson:"municipio"`
	UF           string    `bson:"uf"`
	Instagram    string    `bson:"instagram"`
	Followers    string    `bson:"followers"`
	UpdatedAt    time.Time `bson:"updated_at"`
	ExpiresAt    time.Time `bson:"expires_at"`
}
