package leads

import (
"bytes"
"context"
"encoding/json"
"fmt"
"net/http"
"net/url"
"os"
"strings"
"time"

"github.com/PuerkitoBio/goquery"
)

type GroqScraper struct {
APIKey  string
client  *http.Client
baseURL string
}

func NewGroqScraper(apiKey string) *GroqScraper {
if apiKey == "" {
apiKey = os.Getenv("GROQ_API_KEY")
}
return &GroqScraper{
APIKey:  apiKey,
client:  &http.Client{Timeout: 30 * time.Second},
baseURL: "https://api.groq.com/openai/v1/chat/completions",
}
}

func (g *GroqScraper) Name() string { return "Groq AI" }

func (g *GroqScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
if g.APIKey == "" {
return nil, fmt.Errorf("GROQ_API_KEY nao configurada")
}
city, state := ParseLocation(location)
rawText, err := collectRawSearchText(ctx, query, city, state)
if err != nil || len(rawText) < 100 {
return nil, fmt.Errorf("nao foi possivel coletar texto para Groq: %w", err)
}
maxLen := len(rawText)
if maxLen > 4000 {
maxLen = 4000
}
prompt := fmt.Sprintf("Voce e um extrator de leads de negocios. Analise o texto abaixo e extraia TODOS os estabelecimentos comerciais do tipo \"%s\" em \"%s-%s\".\n\nRetorne APENAS um JSON array com objetos contendo: name, phone, address, website, email.\nUse \"\" para campos nao disponiveis. Sem explicacoes, apenas o JSON.\n\nTexto:\n%s", query, city, state, rawText[:maxLen])
body := map[string]interface{}{
"model": "llama-3.3-70b-versatile",
"messages": []map[string]string{
{"role": "user", "content": prompt},
},
"temperature": 0.1,
"max_tokens":  2048,
}
bodyBytes, _ := json.Marshal(body)
req, err := http.NewRequestWithContext(ctx, "POST", g.baseURL, bytes.NewReader(bodyBytes))
if err != nil {
return nil, err
}
req.Header.Set("Authorization", "Bearer "+g.APIKey)
req.Header.Set("Content-Type", "application/json")
resp, err := g.client.Do(req)
if err != nil {
return nil, err
}
defer resp.Body.Close()
var groqResp struct {
Choices []struct {
Message struct {
Content string `json:"content"`
} `json:"message"`
} `json:"choices"`
}
if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
return nil, err
}
if len(groqResp.Choices) == 0 {
return nil, fmt.Errorf("groq retornou resposta vazia")
}
return parseAILeads(groqResp.Choices[0].Message.Content, city, state, "Groq AI")
}

func collectRawSearchText(ctx context.Context, query, city, state string) (string, error) {
q := fmt.Sprintf("%s %s %s telefone", query, city, state)
searchURL := "https://duckduckgo.com/html/?q=" + url.QueryEscape(q)
req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
if err != nil {
return "", err
}
req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")
time.Sleep(500 * time.Millisecond)
client := &http.Client{Timeout: 15 * time.Second}
resp, err := client.Do(req)
if err != nil {
return "", err
}
defer resp.Body.Close()
if resp.StatusCode != 200 {
return "", fmt.Errorf("ddg status %d", resp.StatusCode)
}
doc, err := goquery.NewDocumentFromReader(resp.Body)
if err != nil {
return "", err
}
var sb strings.Builder
doc.Find(".result__snippet, .result__title, .result__body").Each(func(_ int, s *goquery.Selection) {
t := strings.TrimSpace(s.Text())
if t != "" {
sb.WriteString(t)
sb.WriteString("\n")
}
})
return sb.String(), nil
}

func parseAILeads(content, city, state, source string) ([]*Lead, error) {
content = strings.TrimSpace(content)
for _, pfx := range []string{"```json", "```"} {
content = strings.TrimPrefix(content, pfx)
}
content = strings.TrimSuffix(content, "```")
content = strings.TrimSpace(content)
start := strings.Index(content, "[")
end := strings.LastIndex(content, "]")
if start == -1 || end == -1 || end <= start {
return nil, fmt.Errorf("resposta AI nao contem JSON array valido")
}
var extracted []struct {
Name    string `json:"name"`
Phone   string `json:"phone"`
Address string `json:"address"`
Website string `json:"website"`
Email   string `json:"email"`
}
if err := json.Unmarshal([]byte(content[start:end+1]), &extracted); err != nil {
return nil, fmt.Errorf("erro ao parsear JSON da AI: %w", err)
}
var found []*Lead
for _, e := range extracted {
if e.Name == "" {
continue
}
found = append(found, &Lead{Name: e.Name, Phone: normalizePhone(e.Phone),
Address: e.Address, Website: e.Website, Email: e.Email,
City: city, State: state, Source: source})
}
return found, nil
}
