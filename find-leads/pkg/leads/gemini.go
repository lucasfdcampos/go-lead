package leads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// GeminiScraper usa a API Gemini 1.5-flash (último recurso)
type GeminiScraper struct {
	APIKey  string
	client  *http.Client
	baseURL string
}

func NewGeminiScraper(apiKey string) *GeminiScraper {
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	return &GeminiScraper{
		APIKey:  apiKey,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent",
	}
}

func (g *GeminiScraper) Name() string { return "Gemini AI (last resort)" }

func (g *GeminiScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	if g.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY não configurada")
	}

	city, state := ParseLocation(location)

	// Coleta texto
	rawText, err := collectRawSearchText(ctx, query, city, state)
	if err != nil || len(rawText) < 100 {
		return nil, fmt.Errorf("não foi possível coletar texto para Gemini: %w", err)
	}

	maxLen := len(rawText)
	if maxLen > 4000 {
		maxLen = 4000
	}

	prompt := fmt.Sprintf(`Extraia todos os estabelecimentos comerciais do tipo "%s" em "%s-%s" do texto abaixo.
Retorne apenas um JSON array com campos: name, phone, address, website, email.
Use "" para campos não disponíveis.

Texto:
%s`, query, city, state, rawText[:maxLen])

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.1,
			"maxOutputTokens": 2048,
		},
	}

	bodyBytes, _ := json.Marshal(body)
	reqURL := fmt.Sprintf("%s?key=%s", g.baseURL, g.APIKey)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	time.Sleep(500 * time.Millisecond)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini retornou resposta vazia")
	}

	content := geminiResp.Candidates[0].Content.Parts[0].Text

	// Remove markdown code blocks se houver
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	return parseAILeads(content, city, state, "Gemini AI")
}
