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

// extractLeadsFromText extrai leads de texto não estruturado (snippets de busca)
func extractLeadsFromText(text, city, state, source string) []*Lead {
	var result []*Lead
	phoneRe := regexp.MustCompile(`\(?\d{2}\)?\s*\d{4,5}[-\s]?\d{4}`)
	emailRe := regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || len(line) < 5 {
			continue
		}

		// Detecta se a linha parece conter um nome de empresa
		phones := phoneRe.FindAllString(line, 2)
		emails := emailRe.FindAllString(line, 1)

		if len(phones) > 0 || len(emails) > 0 {
			// Tenta extrair nome da linha ou linha anterior
			lead := &Lead{
				City:   city,
				State:  state,
				Source: source,
			}

			if len(phones) > 0 {
				lead.Phone = normalizePhone(phones[0])
				if len(phones) > 1 {
					lead.Phone2 = normalizePhone(phones[1])
				}
			}

			if len(emails) > 0 {
				lead.Email = emails[0]
			}

			// Remove telefones e e-mails para extrair possível nome
			namePart := phoneRe.ReplaceAllString(line, "")
			namePart = emailRe.ReplaceAllString(namePart, "")
			namePart = regexp.MustCompile(`https?://\S+`).ReplaceAllString(namePart, "")
			namePart = strings.TrimSpace(regexp.MustCompile(`[-|·•,]+$`).ReplaceAllString(namePart, ""))

			if len(namePart) > 3 && len(namePart) < 100 {
				lead.Name = namePart
			}

			result = append(result, lead)
		}
	}

	return result
}

// ─── DuckDuckGo ──────────────────────────────────────────────────────────────

type DDGLeadScraper struct{}

func NewDDGLeadScraper() *DDGLeadScraper { return &DDGLeadScraper{} }
func (d *DDGLeadScraper) Name() string   { return "DuckDuckGo" }

func (d *DDGLeadScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	q := fmt.Sprintf(`"%s" "%s" telefone endereço`, query, city)
	return searchEngineLeads(ctx, "https://duckduckgo.com/html/?q="+url.QueryEscape(q),
		".result__snippet, .result__body", city, state, "DuckDuckGo", 1*time.Second)
}

// ─── Bing ────────────────────────────────────────────────────────────────────

type BingLeadScraper struct{}

func NewBingLeadScraper() *BingLeadScraper { return &BingLeadScraper{} }
func (b *BingLeadScraper) Name() string    { return "Bing" }

func (b *BingLeadScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	q := fmt.Sprintf(`"%s" "%s" "%s" telefone`, query, city, state)
	return searchEngineLeads(ctx, "https://www.bing.com/search?q="+url.QueryEscape(q),
		".b_caption p, .b_snippet, .b_dList", city, state, "Bing", 1*time.Second)
}

// ─── Brave ───────────────────────────────────────────────────────────────────

type BraveLeadScraper struct{}

func NewBraveLeadScraper() *BraveLeadScraper { return &BraveLeadScraper{} }
func (b *BraveLeadScraper) Name() string     { return "Brave Search" }

func (b *BraveLeadScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	q := fmt.Sprintf(`%s %s telefone endereço`, query, city)
	return searchEngineLeads(ctx, "https://search.brave.com/search?q="+url.QueryEscape(q),
		".snippet-description, .snippet-content, .result-description", city, state, "Brave", 1*time.Second)
}

// ─── Yandex ──────────────────────────────────────────────────────────────────

type YandexLeadScraper struct{}

func NewYandexLeadScraper() *YandexLeadScraper { return &YandexLeadScraper{} }
func (y *YandexLeadScraper) Name() string      { return "Yandex" }

func (y *YandexLeadScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	q := fmt.Sprintf(`%s %s telefone Brasil`, query, city)
	return searchEngineLeads(ctx,
		fmt.Sprintf("https://yandex.com/search/?text=%s&lr=102", url.QueryEscape(q)),
		".Organic-Text, .OrganicText, .ExtendedText, .serp-item__text", city, state, "Yandex", 1*time.Second)
}

// ─── Helper genérico ─────────────────────────────────────────────────────────

func searchEngineLeads(ctx context.Context, searchURL, selector, city, state, source string, delay time.Duration) ([]*Lead, error) {
	time.Sleep(delay)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

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

	var allText strings.Builder
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		allText.WriteString(s.Text())
		allText.WriteString("\n")
	})

	// Fallback: pega todo o texto visível
	if allText.Len() < 50 {
		doc.Find("p, li, span, div").Each(func(i int, s *goquery.Selection) {
			t := strings.TrimSpace(s.Text())
			if len(t) > 10 && len(t) < 300 {
				allText.WriteString(t)
				allText.WriteString("\n")
			}
		})
	}

	leads := extractLeadsFromText(allText.String(), city, state, source)
	return leads, nil
}
