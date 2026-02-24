package leads

import (
"context"
"encoding/json"
"fmt"
"io"
"net/http"
"net/url"
"os"
"strconv"
"strings"
"time"
)

const (
tomtomBaseURL  = "https://api.tomtom.com/search/2/search"
tomtomTimeout  = 12 * time.Second
tomtomMaxLimit = 50
)

// TomTomScraper busca leads via TomTom Fuzzy Search API.
// Plano gratuito: 2.500 req/dia (~75.000/mês), sem cartão de crédito.
type TomTomScraper struct {
apiKey string
client *http.Client
}

func NewTomTomScraper(apiKey string) *TomTomScraper {
if apiKey == "" {
apiKey = os.Getenv("TOMTOM_API_KEY")
}
return &TomTomScraper{
apiKey: apiKey,
client: &http.Client{Timeout: tomtomTimeout},
}
}

func (t *TomTomScraper) Name() string { return "TomTom Places" }

func (t *TomTomScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
if t.apiKey == "" {
return nil, fmt.Errorf("TOMTOM_API_KEY não configurada")
}

city, state := ParseLocation(location)

// Tenta geocodificar usando Geoapify (se chave disponível) para passar lat/lon ao TomTom
var lat, lon float64
var hasCoords bool
geoKey := os.Getenv("GEOAPIFY_API_KEY")
if geoKey != "" {
var err error
lat, lon, err = geoapifyGeocode(ctx, geoKey, city, state)
if err == nil {
hasCoords = true
}
}

// Monta URL: /search/2/search/{query}.json
fullQuery := query + " " + city + " " + state
reqURL := fmt.Sprintf("%s/%s.json", tomtomBaseURL, url.PathEscape(fullQuery))

params := url.Values{}
params.Set("key", t.apiKey)
params.Set("countrySet", "BR")
params.Set("limit", strconv.Itoa(tomtomMaxLimit))
params.Set("language", "pt-BR")
params.Set("typeahead", "false")

if hasCoords {
params.Set("lat", strconv.FormatFloat(lat, 'f', 6, 64))
params.Set("lon", strconv.FormatFloat(lon, 'f', 6, 64))
params.Set("radius", "15000") // 15 km ao redor da cidade
}

fullURL := reqURL + "?" + params.Encode()

req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
if err != nil {
return nil, fmt.Errorf("tomtom: build request: %w", err)
}
req.Header.Set("Accept", "application/json")

resp, err := t.client.Do(req)
if err != nil {
return nil, fmt.Errorf("tomtom: request failed: %w", err)
}
defer resp.Body.Close()

if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
return nil, fmt.Errorf("tomtom: chave API inválida (status %d)", resp.StatusCode)
}
if resp.StatusCode != http.StatusOK {
return nil, fmt.Errorf("tomtom: HTTP %d", resp.StatusCode)
}

body, err := io.ReadAll(resp.Body)
if err != nil {
return nil, fmt.Errorf("tomtom: read body: %w", err)
}

var ttResp tomtomResponse
if err := json.Unmarshal(body, &ttResp); err != nil {
return nil, fmt.Errorf("tomtom: unmarshal: %w", err)
}

var leads []*Lead
for _, r := range ttResp.Results {
if r.POI.Name == "" {
continue
}

phone := normalizePhone(r.POI.Phone)

website := r.POI.URL
if website != "" && !strings.HasPrefix(website, "http") {
website = "https://" + website
}

addr := r.Address.StreetName
if r.Address.FreeformAddress != "" && addr == "" {
addr = r.Address.FreeformAddress
}

resultCity := r.Address.Municipality
if resultCity == "" {
resultCity = city
}
resultState := r.Address.CountrySubdivision
if resultState == "" {
resultState = state
}

leads = append(leads, &Lead{
Name:    r.POI.Name,
Phone:   phone,
Website: website,
Address: addr,
City:    resultCity,
State:   resultState,
Source:  "TomTom Places",
})
}

return leads, nil
}

// ─── Structs de resposta ──────────────────────────────────────────────────────

type tomtomResponse struct {
Results []tomtomResult `json:"results"`
}

type tomtomResult struct {
POI     tomtomPOI     `json:"poi"`
Address tomtomAddress `json:"address"`
}

type tomtomPOI struct {
Name  string `json:"name"`
Phone string `json:"phone"`
URL   string `json:"url"`
}

type tomtomAddress struct {
StreetName          string `json:"streetName"`
Municipality        string `json:"municipality"`
CountrySubdivision  string `json:"countrySubdivision"`
PostalCode          string `json:"postalCode"`
FreeformAddress     string `json:"freeformAddress"`
}
