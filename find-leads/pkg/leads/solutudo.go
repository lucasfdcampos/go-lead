package leads

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// SolutudoScraper busca leads no Solutudo
type SolutudoScraper struct{}

func NewSolutudoScraper() *SolutudoScraper { return &SolutudoScraper{} }
func (s *SolutudoScraper) Name() string    { return "Solutudo" }

func (s *SolutudoScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	stateSlug := strings.ToLower(state)
	citySlug := CitySlug(city)
	querySlug := QuerySlug(query)

	// Tenta múltiplas variações de URL
	urls := []string{
		fmt.Sprintf("https://www.solutudo.com.br/empresas/%s/%s/%s", stateSlug, citySlug, querySlug),
		fmt.Sprintf("https://www.solutudo.com.br/empresas/%s/%s", citySlug, querySlug),
		fmt.Sprintf("https://www.solutudo.com.br/empresas/%s", querySlug),
	}

	var leads []*Lead
	for _, url := range urls {
		found, err := scrapeSolutudo(ctx, url, city, state)
		if err == nil && len(found) > 0 {
			leads = append(leads, found...)
			break
		}
		time.Sleep(800 * time.Millisecond)
	}

	return leads, nil
}

func scrapeSolutudo(ctx context.Context, url, city, state string) ([]*Lead, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

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

	// Solutudo usa cards de empresa
	doc.Find(".company-card, .empresa-card, .empresa-item, .listing-item, article").Each(func(i int, sel *goquery.Selection) {
		lead := &Lead{City: city, State: state, Source: "Solutudo"}

		lead.Name = strings.TrimSpace(sel.Find("h2, h3, .company-name, .nome, .name, a[href]").First().Text())
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

		lead.Address = strings.TrimSpace(sel.Find(".address, .endereco, .logradouro").Text())
		lead.Category = strings.TrimSpace(sel.Find(".category, .categoria, .segmento").Text())

		if href, ok := sel.Find("a").First().Attr("href"); ok && strings.HasPrefix(href, "http") {
			lead.Website = href
		}

		if lead.Name != "" && len(lead.Name) > 2 {
			leads = append(leads, lead)
		}
	})

	return leads, nil
}

func normalizePhone(phone string) string {
	re := regexp.MustCompile(`\D`)
	digits := re.ReplaceAllString(phone, "")
	if len(digits) == 10 {
		return fmt.Sprintf("(%s) %s-%s", digits[0:2], digits[2:6], digits[6:10])
	} else if len(digits) == 11 {
		return fmt.Sprintf("(%s) %s-%s", digits[0:2], digits[2:7], digits[7:11])
	}
	return phone
}
