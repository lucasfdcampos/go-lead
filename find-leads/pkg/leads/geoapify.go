package leads

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// GeoapifyScraper busca estabelecimentos via Geoapify Places API
type GeoapifyScraper struct {
	APIKey string
}

func NewGeoapifyScraper(apiKey string) *GeoapifyScraper {
	if apiKey == "" {
		apiKey = os.Getenv("GEOAPIFY_API_KEY")
	}
	return &GeoapifyScraper{APIKey: apiKey}
}

func (g *GeoapifyScraper) Name() string { return "Geoapify Places" }

// queryParaGeoapifyCategory converte query para categoria Geoapify
func queryParaGeoapifyCategory(query string) string {
	q := strings.ToLower(query)
	switch {
	case contains(q, "roupa", "vestuario", "moda", "confeccao"):
		return "commercial.clothing"
	case contains(q, "calcado", "sapato"):
		return "commercial.clothing.shoes"
	case contains(q, "farmacia", "drogaria"):
		return "healthcare.pharmacy"
	case contains(q, "restaurante", "lanchonete"):
		return "catering.restaurant"
	case contains(q, "supermercado", "mercado"):
		return "commercial.supermarket"
	case contains(q, "academia", "fitness"):
		return "leisure.fitness"
	case contains(q, "banco"):
		return "commercial.financial.bank"
	case contains(q, "padaria"):
		return "catering.fast_food.bakery"
	case contains(q, "barbearia", "cabeleireiro", "salao"):
		return "service.beauty.hairdresser"
	case contains(q, "pet", "veterinaria"):
		return "service.animal_shelter"
	default:
		return "commercial"
	}
}

func (g *GeoapifyScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	if g.APIKey == "" {
		return nil, fmt.Errorf("GEOAPIFY_API_KEY não configurada")
	}

	city, state := ParseLocation(location)

	// Geocodifica a cidade
	lat, lon, err := geoapifyGeocode(ctx, g.APIKey, city, state)
	if err != nil {
		return nil, fmt.Errorf("geocodificação falhou: %w", err)
	}

	category := queryParaGeoapifyCategory(query)

	// Busca places (raio de 10km)
	placesURL := fmt.Sprintf(
		"https://api.geoapify.com/v2/places?categories=%s&filter=circle:%f,%f,10000&limit=500&apiKey=%s",
		url.QueryEscape(category), lon, lat, g.APIKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", placesURL, nil)
	if err != nil {
		return nil, err
	}

	time.Sleep(300 * time.Millisecond)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("geoapify places status %d", resp.StatusCode)
	}

	var result struct {
		Features []struct {
			Properties struct {
				Name       string   `json:"name"`
				Phone      string   `json:"phone"`
				Website    string   `json:"website"`
				Email      string   `json:"email"`
				Street     string   `json:"street"`
				HouseNum   string   `json:"housenumber"`
				City       string   `json:"city"`
				Categories []string `json:"categories"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var leadsFound []*Lead
	for _, f := range result.Features {
		p := f.Properties
		if p.Name == "" {
			continue
		}

		lead := &Lead{
			Name:    p.Name,
			Phone:   normalizePhone(p.Phone),
			Website: p.Website,
			Email:   p.Email,
			City:    city,
			State:   state,
			Source:  "Geoapify",
		}

		if p.Street != "" {
			lead.Address = p.Street
			if p.HouseNum != "" {
				lead.Address += ", " + p.HouseNum
			}
		}

		if len(p.Categories) > 0 {
			lead.Category = p.Categories[0]
		}

		leadsFound = append(leadsFound, lead)
	}

	return leadsFound, nil
}

// geoapifyGeocode retorna lat, lon de uma cidade via Geoapify Geocoding
func geoapifyGeocode(ctx context.Context, apiKey, city, state string) (float64, float64, error) {
	text := fmt.Sprintf("%s, %s, Brazil", city, state)
	reqURL := fmt.Sprintf(
		"https://api.geoapify.com/v1/geocode/search?text=%s&format=json&apiKey=%s",
		url.QueryEscape(text), apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return 0, 0, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Results) == 0 {
		return 0, 0, fmt.Errorf("geocode não retornou resultados")
	}

	return result.Results[0].Lat, result.Results[0].Lon, nil
}
