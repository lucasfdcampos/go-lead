package leads

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OverpassScraper busca estabelecimentos via OpenStreetMap Overpass API (gratuito)
type OverpassScraper struct{}

func NewOverpassScraper() *OverpassScraper { return &OverpassScraper{} }
func (o *OverpassScraper) Name() string    { return "OpenStreetMap (Overpass)" }

// categoriaParaOSM converte query para tags OSM relevantes
func categoriaParaOSM(query string) []string {
	q := strings.ToLower(query)
	tags := []string{}

	switch {
	case contains(q, "roupa", "vestuario", "moda", "confeccao", "outfit"):
		tags = append(tags, `"shop"="clothes"`, `"shop"="boutique"`, `"shop"="fashion"`)
	case contains(q, "calcado", "sapato", "tenis"):
		tags = append(tags, `"shop"="shoes"`)
	case contains(q, "farmacia", "drogaria"):
		tags = append(tags, `"amenity"="pharmacy"`)
	case contains(q, "restaurante", "lanchonete", "comida"):
		tags = append(tags, `"amenity"="restaurant"`, `"amenity"="fast_food"`, `"amenity"="cafe"`)
	case contains(q, "supermercado", "mercado", "mercadinho"):
		tags = append(tags, `"shop"="supermarket"`, `"shop"="convenience"`)
	case contains(q, "academia", "fitness", "musculacao"):
		tags = append(tags, `"leisure"="fitness_centre"`, `"leisure"="sports_centre"`)
	case contains(q, "banco", "financeira"):
		tags = append(tags, `"amenity"="bank"`)
	case contains(q, "padaria", "confeitaria"):
		tags = append(tags, `"shop"="bakery"`)
	case contains(q, "barbearia", "cabeleireiro", "salao"):
		tags = append(tags, `"shop"="hairdresser"`, `"shop"="barber"`)
	case contains(q, "pet", "veterinaria", "animal"):
		tags = append(tags, `"shop"="pet"`, `"amenity"="veterinary"`)
	default:
		tags = append(tags, `"shop"`, `"amenity"`)
	}

	return tags
}

func contains(s string, keywords ...string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func (o *OverpassScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)

	// Primeiro geocodifica a cidade via Nominatim para obter bbox
	bbox, err := nominatimBBox(ctx, city, state)
	if err != nil {
		return nil, fmt.Errorf("erro geocodificando cidade: %w", err)
	}

	tags := categoriaParaOSM(query)
	if len(tags) == 0 {
		return nil, fmt.Errorf("nenhuma tag OSM mapeada para query")
	}

	// Monta query Overpass
	var nodeBlocks strings.Builder
	for _, tag := range tags {
		nodeBlocks.WriteString(fmt.Sprintf(`node[%s](%s);`, tag, bbox))
		nodeBlocks.WriteString(fmt.Sprintf(`way[%s](%s);`, tag, bbox))
	}

	overpassQuery := fmt.Sprintf(`[out:json][timeout:30];(%s);out body;`, nodeBlocks.String())

	reqURL := "https://overpass-api.de/api/interpreter?data=" + url.QueryEscape(overpassQuery)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "find-leads/1.0 (business lead finder)")

	time.Sleep(1 * time.Second)

	client := &http.Client{Timeout: 35 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("overpass status %d", resp.StatusCode)
	}

	var result struct {
		Elements []struct {
			Type string `json:"type"`
			ID   int64  `json:"id"`
			Tags struct {
				Name    string `json:"name"`
				Phone   string `json:"phone"`
				Phone2  string `json:"phone:2"`
				Website string `json:"website"`
				Email   string `json:"email"`
				Street  string `json:"addr:street"`
				HouseNr string `json:"addr:housenumber"`
				City    string `json:"addr:city"`
				Shop    string `json:"shop"`
				Amenity string `json:"amenity"`
			} `json:"tags"`
		} `json:"elements"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var leads []*Lead
	for _, el := range result.Elements {
		if el.Tags.Name == "" {
			continue
		}

		lead := &Lead{
			Name:    el.Tags.Name,
			Phone:   normalizePhone(el.Tags.Phone),
			Phone2:  normalizePhone(el.Tags.Phone2),
			Website: el.Tags.Website,
			Email:   el.Tags.Email,
			City:    city,
			State:   state,
			Source:  "OpenStreetMap",
		}

		if el.Tags.Street != "" {
			lead.Address = el.Tags.Street
			if el.Tags.HouseNr != "" {
				lead.Address += ", " + el.Tags.HouseNr
			}
		}

		if el.Tags.Shop != "" {
			lead.Category = el.Tags.Shop
		} else if el.Tags.Amenity != "" {
			lead.Category = el.Tags.Amenity
		}

		leads = append(leads, lead)
	}

	return leads, nil
}

// nominatimBBox retorna "sul,oeste,norte,leste" para a cidade via Nominatim
func nominatimBBox(ctx context.Context, city, state string) (string, error) {
	searchQuery := fmt.Sprintf("%s, %s, Brazil", city, state)
	reqURL := fmt.Sprintf("https://nominatim.openstreetmap.org/search?q=%s&format=json&limit=1",
		url.QueryEscape(searchQuery))

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "find-leads/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var results []struct {
		BoundingBox []string `json:"boundingbox"`
		Lat         string   `json:"lat"`
		Lon         string   `json:"lon"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil || len(results) == 0 {
		return "", fmt.Errorf("cidade não encontrada no Nominatim")
	}

	bb := results[0].BoundingBox
	if len(bb) < 4 {
		// Fallback: bbox 0.05 graus em volta do centro
		lat := results[0].Lat
		lon := results[0].Lon
		return fmt.Sprintf("%s,%s,%s,%s",
			subFloat(lat, 0.05),
			subFloat(lon, 0.05),
			addFloat(lat, 0.05),
			addFloat(lon, 0.05),
		), nil
	}

	// Nominatim retorna [minlat, maxlat, minlon, maxlon]
	return fmt.Sprintf("%s,%s,%s,%s", bb[0], bb[2], bb[1], bb[3]), nil
}

func subFloat(s string, delta float64) string {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return fmt.Sprintf("%f", f-delta)
}

func addFloat(s string, delta float64) string {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return fmt.Sprintf("%f", f+delta)
}
