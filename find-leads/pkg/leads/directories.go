package leads

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ApontadorScraper busca leads no Apontador
type ApontadorScraper struct{}

func NewApontadorScraper() *ApontadorScraper { return &ApontadorScraper{} }
func (a *ApontadorScraper) Name() string     { return "Apontador" }

func (a *ApontadorScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)

	searchURL := fmt.Sprintf(
		"https://www.apontador.com.br/local/busca/?q=%s&where=%s+%s",
		url.QueryEscape(query),
		url.QueryEscape(city),
		url.QueryEscape(state),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	time.Sleep(500 * time.Millisecond)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var leads []*Lead
	phoneRe := regexp.MustCompile(`\(?\d{2}\)?\s*\d{4,5}[-\s]?\d{4}`)

	doc.Find(".place-item, .result-item, .listing-item, .local-item, li.item").Each(func(i int, sel *goquery.Selection) {
		lead := &Lead{City: city, State: state, Source: "Apontador"}

		lead.Name = strings.TrimSpace(sel.Find("h2, h3, h4, .local-name, .place-name, .titulo").First().Text())
		if lead.Name == "" {
			lead.Name = strings.TrimSpace(sel.Find("a").First().Text())
		}

		text := sel.Text()
		if phones := phoneRe.FindAllString(text, 2); len(phones) > 0 {
			lead.Phone = normalizePhone(phones[0])
			if len(phones) > 1 {
				lead.Phone2 = normalizePhone(phones[1])
			}
		}

		lead.Address = strings.TrimSpace(sel.Find(".local-address, .address, .endereco").Text())
		lead.Category = strings.TrimSpace(sel.Find(".local-category, .categoria, .tipo").Text())

		if rating := sel.Find(".rating, .nota, .avaliacao").Text(); rating != "" {
			lead.Rating = strings.TrimSpace(rating)
		}

		if lead.Name != "" && len(lead.Name) > 2 {
			leads = append(leads, lead)
		}
	})

	return leads, nil
}

// TeleListasScraper busca no TeleListas (lista telefônica)
type TeleListasScraper struct{}

func NewTeleListasScraper() *TeleListasScraper { return &TeleListasScraper{} }
func (t *TeleListasScraper) Name() string      { return "TeleListas" }

func (t *TeleListasScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	stateSlug := strings.ToLower(state)
	citySlug := CitySlug(city)

	searchURL := fmt.Sprintf("http://www.telelistas.net/%s/%s/%s",
		stateSlug, citySlug, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	time.Sleep(500 * time.Millisecond)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var leads []*Lead
	phoneRe := regexp.MustCompile(`\(?\d{2}\)?\s*\d{4,5}[-\s]?\d{4}`)

	doc.Find(".results li, .lista-empresas li, .empresa-item, .item").Each(func(i int, sel *goquery.Selection) {
		lead := &Lead{City: city, State: state, Source: "TeleListas"}

		lead.Name = strings.TrimSpace(sel.Find("h2, h3, strong, .nome").First().Text())
		if lead.Name == "" {
			lead.Name = strings.TrimSpace(sel.Find("a").First().Text())
		}

		text := sel.Text()
		if phones := phoneRe.FindAllString(text, 2); len(phones) > 0 {
			lead.Phone = normalizePhone(phones[0])
			if len(phones) > 1 {
				lead.Phone2 = normalizePhone(phones[1])
			}
		}

		lead.Address = strings.TrimSpace(sel.Find(".address, .endereco").Text())

		if lead.Name != "" && len(lead.Name) > 2 {
			leads = append(leads, lead)
		}
	})

	return leads, nil
}
