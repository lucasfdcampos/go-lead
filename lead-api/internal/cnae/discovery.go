// Package cnae – discovery.go
// Discovers CNAE codes for a given query by scraping DuckDuckGo search results.
// The result is cached in MongoDB (cnae_hints collection) to avoid repeated lookups.
package cnae

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

// cnaeCodeRe matches CNAE code patterns like 4781-4/00, 47.81-4/00, 4781, 47814
var cnaeCodeRe = regexp.MustCompile(`\b(\d{4}[\d\.\-\/]{0,5})\b`)

// DiscoverFromSearch searches DuckDuckGo for `"<query>" CNAE Brasil` and extracts
// all CNAE codes found in the results snippets.
// Returns (codes, rawSnippet, error). On failure it returns empty codes (no error) so the
// pipeline can fall back to the static map gracefully.
func DiscoverFromSearch(ctx context.Context, query string) (codes []string, snippet string, err error) {
	q := fmt.Sprintf(`"%s" CNAE Brasil atividade econômica`, query)
	searchURL := "https://lite.duckduckgo.com/lite/?q=" + url.QueryEscape(q)

	tctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(tctx, "GET", searchURL, nil)
	if err != nil {
		return nil, "", nil //nolint – non-fatal
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	time.Sleep(800 * time.Millisecond)

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, "", nil
	}

	var sb strings.Builder
	doc.Find("td.result-snippet, .result-snippet, td").Each(func(_ int, s *goquery.Selection) {
		sb.WriteString(s.Text())
		sb.WriteString("\n")
	})
	raw := sb.String()

	// Additionally try Mojeek as a second source
	mojeekSnippet := discoverFromMojeek(ctx, query)
	if mojeekSnippet != "" {
		raw += "\n" + mojeekSnippet
	}

	codes = extractCNAECodes(raw)
	return codes, raw, nil
}

// discoverFromMojeek performs a CNAE discovery search on Mojeek.
func discoverFromMojeek(ctx context.Context, query string) string {
	q := fmt.Sprintf(`%s CNAE atividade`, query)
	searchURL := "https://www.mojeek.com/search?q=" + url.QueryEscape(q)

	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(tctx, "GET", searchURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	time.Sleep(500 * time.Millisecond)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return ""
	}

	var sb strings.Builder
	doc.Find(".result-text, .result__body, p").Each(func(_ int, s *goquery.Selection) {
		sb.WriteString(s.Text())
		sb.WriteString("\n")
	})
	return sb.String()
}

// extractCNAECodes parses raw text and returns unique 4-digit CNAE code prefixes.
// It focuses on 4-digit groups that look like CNAE codes (XXXX or XXXX-X/XX format).
func extractCNAECodes(text string) []string {
	// First try explicit "CNAE: XXXX..." patterns
	explicitRe := regexp.MustCompile(`(?i)cnae\s*:?\s*([\d]{4}[\d\.\-\/]*)`)
	atividadeRe := regexp.MustCompile(`(?i)atividade\s+principal\s*:?\s*([\d]{4}[\d\.\-\/]*)`)
	classRe := regexp.MustCompile(`(?i)classe\s*([\d]{4}[\d\.\-\/]*)`)

	seen := make(map[string]bool)
	var codes []string

	addCode := func(raw string) {
		// Extract just the 4-digit prefix
		digits := regexp.MustCompile(`\D`).ReplaceAllString(raw, "")
		if len(digits) >= 4 {
			prefix := digits[:4]
			if !seen[prefix] {
				seen[prefix] = true
				codes = append(codes, prefix)
			}
		}
	}

	for _, re := range []*regexp.Regexp{explicitRe, atividadeRe, classRe} {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			if len(m) >= 2 {
				addCode(m[1])
			}
		}
	}

	// Fallback: look for 4-digit sequences near keywords
	nearRe := regexp.MustCompile(`(?i)(?:cnae|atividade|classe|código)\D{0,10}(\d{4})`)
	for _, m := range nearRe.FindAllStringSubmatch(text, -1) {
		if len(m) >= 2 {
			addCode(m[1])
		}
	}

	return codes
}
