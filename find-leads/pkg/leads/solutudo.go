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
	// Valida DDD brasileiro (11-19, 21-24, 27-28, 31-38, 41-47, 49, 51-55, 61-69, 71-77, 79, 81-89, 91-99)
	validDDD := map[string]bool{
		"11": true, "12": true, "13": true, "14": true, "15": true, "16": true, "17": true, "18": true, "19": true,
		"21": true, "22": true, "23": true, "24": true, "27": true, "28": true,
		"31": true, "32": true, "33": true, "34": true, "35": true, "36": true, "37": true, "38": true,
		"41": true, "42": true, "43": true, "44": true, "45": true, "46": true, "47": true, "49": true,
		"51": true, "53": true, "54": true, "55": true,
		"61": true, "62": true, "63": true, "64": true, "65": true, "66": true, "67": true, "68": true, "69": true,
		"71": true, "73": true, "74": true, "75": true, "77": true, "79": true,
		"81": true, "82": true, "83": true, "84": true, "85": true, "86": true, "87": true, "88": true, "89": true,
		"91": true, "92": true, "93": true, "94": true, "95": true, "96": true, "97": true, "98": true, "99": true,
	}
	if len(digits) >= 10 {
		ddd := digits[0:2]
		if !validDDD[ddd] {
			return "" // DDD inválido — provavelmente número de CNPJ
		}
		// Rejeita números com 4 dígitos finais iguais (ex: 1111, 2222) — provável dado inventado
		last4 := digits[len(digits)-4:]
		if last4 == "0000" || last4 == "1111" || last4 == "2222" || last4 == "3333" ||
			last4 == "4444" || last4 == "5555" || last4 == "6666" || last4 == "7777" ||
			last4 == "8888" || last4 == "9999" {
			return ""
		}
	}
	if len(digits) == 10 {
		return fmt.Sprintf("(%s) %s-%s", digits[0:2], digits[2:6], digits[6:10])
	} else if len(digits) == 11 {
		return fmt.Sprintf("(%s) %s-%s", digits[0:2], digits[2:7], digits[7:11])
	}
	return phone
}
