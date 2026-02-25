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

// GuiaMaisScraper busca leads no GuiaMais
type GuiaMaisScraper struct{}

func NewGuiaMaisScraper() *GuiaMaisScraper { return &GuiaMaisScraper{} }
func (g *GuiaMaisScraper) Name() string    { return "GuiaMais" }

func (g *GuiaMaisScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	cityState := fmt.Sprintf("%s-%s", CitySlug(city), strings.ToLower(state))
	querySlug := QuerySlug(query)

	urls := []string{
		fmt.Sprintf("https://www.guiamais.com.br/%s/%s", cityState, querySlug),
		fmt.Sprintf("https://www.guiamais.com.br/buscar?q=%s&where=%s",
			url.QueryEscape(query), url.QueryEscape(location)),
	}

	var allLeads []*Lead
	for _, u := range urls {
		found, err := scrapeGuiaMais(ctx, u, city, state)
		if err == nil && len(found) > 0 {
			allLeads = append(allLeads, found...)
			break
		}
		time.Sleep(800 * time.Millisecond)
	}
	return allLeads, nil
}

func scrapeGuiaMais(ctx context.Context, rawURL, city, state string) ([]*Lead, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := DoWithRetry(ctx, client, req, 3)
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

	doc.Find(".listing, .result-item, .company-item, .empresa, li.item").Each(func(i int, sel *goquery.Selection) {
		lead := &Lead{City: city, State: state, Source: "GuiaMais"}

		lead.Name = strings.TrimSpace(sel.Find("h2, h3, h4, .title, .nome, strong").First().Text())
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

		lead.Address = strings.TrimSpace(sel.Find(".address, .endereco, address").Text())
		lead.Category = strings.TrimSpace(sel.Find(".category, .atividade, .ramo").Text())

		if href, ok := sel.Find("a[href]").First().Attr("href"); ok {
			if strings.HasPrefix(href, "http") {
				lead.Website = href
			}
		}

		if lead.Name != "" && len(lead.Name) > 2 {
			leads = append(leads, lead)
		}
	})

	return leads, nil
}
