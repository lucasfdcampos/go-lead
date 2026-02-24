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

// AppLocalScraper busca leads no AppLocal
type AppLocalScraper struct{}

func NewAppLocalScraper() *AppLocalScraper { return &AppLocalScraper{} }
func (a *AppLocalScraper) Name() string    { return "AppLocal" }

func (a *AppLocalScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	cityState := fmt.Sprintf("%s-%s", CitySlug(city), strings.ToLower(state))
	querySlug := QuerySlug(query)

	urls := []string{
		fmt.Sprintf("https://applocal.com.br/empresas/%s/%s", cityState, querySlug),
		fmt.Sprintf("https://applocal.com.br/empresas/%s/%s", CitySlug(city), querySlug),
		fmt.Sprintf("https://applocal.com.br/busca?q=%s&cidade=%s",
			url.QueryEscape(query), url.QueryEscape(city)),
	}

	var allLeads []*Lead
	for _, u := range urls {
		found, err := scrapeAppLocal(ctx, u, city, state)
		if err == nil && len(found) > 0 {
			allLeads = append(allLeads, found...)
			break
		}
		time.Sleep(800 * time.Millisecond)
	}
	return allLeads, nil
}

func scrapeAppLocal(ctx context.Context, rawURL, city, state string) ([]*Lead, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
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

	var leads []*Lead
	phoneRe := regexp.MustCompile(`\(?\d{2}\)?\s*\d{4,5}[-\s]?\d{4}`)

	doc.Find("article, .card, .empresa-card, .business-card, .item-lista").Each(func(i int, sel *goquery.Selection) {
		lead := &Lead{City: city, State: state, Source: "AppLocal"}

		lead.Name = strings.TrimSpace(sel.Find("h1, h2, h3, .nome, .title").First().Text())
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

		lead.Address = strings.TrimSpace(sel.Find(".endereco, .address, .localizacao").Text())

		if lead.Name != "" && len(lead.Name) > 2 {
			leads = append(leads, lead)
		}
	})

	return leads, nil
}
