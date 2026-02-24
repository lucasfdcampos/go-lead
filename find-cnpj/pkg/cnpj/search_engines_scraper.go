package cnpj

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// DuckDuckGoScraper busca dados de CNPJ via snippets de busca do DuckDuckGo
type DuckDuckGoScraper struct{}

func NewDuckDuckGoScraper() *DuckDuckGoScraper {
	return &DuckDuckGoScraper{}
}

func (d *DuckDuckGoScraper) Name() string {
	return "DuckDuckGo Search"
}

// EnrichFromDuckDuckGo busca dados de sócios via DuckDuckGo
func EnrichFromDuckDuckGo(ctx context.Context, cnpj *CNPJ) error {
	if cnpj == nil || cnpj.Number == "" {
		return fmt.Errorf("CNPJ inválido")
	}

	// Tenta buscar por CNPJ
	socios, razaoSocial, telefones := searchDuckDuckGo(ctx, cnpj.Number, "cnpj")

	// Se não conseguiu muita coisa e tem razão social, busca por ela
	if len(socios) == 0 && cnpj.RazaoSocial != "" {
		sociosRS, razaoRS, telefonesRS := searchDuckDuckGo(ctx, cnpj.RazaoSocial, "razao-social")

		// Merge resultados
		socios = append(socios, sociosRS...)
		if razaoSocial == "" && razaoRS != "" {
			razaoSocial = razaoRS
		}
		telefones = append(telefones, telefonesRS...)
	}

	// Atualiza dados do CNPJ
	updated := false

	if razaoSocial != "" && cnpj.RazaoSocial == "" {
		cnpj.RazaoSocial = razaoSocial
		updated = true
	}

	if len(socios) > 0 {
		// Remove duplicatas
		sociosMap := make(map[string]bool)
		for _, s := range cnpj.Socios {
			sociosMap[strings.ToLower(s)] = true
		}

		for _, s := range socios {
			if !sociosMap[strings.ToLower(s)] {
				cnpj.Socios = append(cnpj.Socios, s)
				updated = true
			}
		}
	}

	if len(telefones) > 0 {
		// Remove duplicatas
		telefonesMap := make(map[string]bool)
		for _, t := range cnpj.Telefones {
			telefonesMap[t] = true
		}

		for _, t := range telefones {
			if !telefonesMap[t] {
				cnpj.Telefones = append(cnpj.Telefones, t)
				updated = true
			}
		}
	}

	if !updated {
		return fmt.Errorf("nenhum dado novo encontrado no DuckDuckGo")
	}

	return nil
}

// searchDuckDuckGo faz busca no DuckDuckGo e extrai informações
func searchDuckDuckGo(ctx context.Context, query string, searchType string) (socios []string, razaoSocial string, telefones []string) {
	var searchQuery string

	if searchType == "cnpj" {
		searchQuery = fmt.Sprintf("%s sócios administradores", query)
	} else {
		searchQuery = fmt.Sprintf("%s CNPJ sócios", query)
	}

	url := fmt.Sprintf("https://duckduckgo.com/html/?q=%s", searchQuery)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	time.Sleep(1 * time.Second)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return
	}

	// Extrai de snippets de resultados
	doc.Find(".result__snippet, .result-snippet, .result__body").Each(func(i int, s *goquery.Selection) {
		text := s.Text()

		// Busca razão social se ainda não temos
		if razaoSocial == "" {
			if rs := extractRazaoSocial(text); rs != "" {
				razaoSocial = rs
			}
		}

		// Busca sócios
		foundSocios := extractSocios(text)
		socios = append(socios, foundSocios...)

		// Busca telefones
		foundTelefones := extractTelefonesFromText(text)
		telefones = append(telefones, foundTelefones...)
	})

	// Remove duplicatas de sócios
	socios = removeDuplicates(socios)
	telefones = removeDuplicates(telefones)

	return
}

// extractRazaoSocial tenta extrair razão social de um texto
func extractRazaoSocial(text string) string {
	// Padrões comuns:
	// "Razão Social: EMPRESA LTDA"
	// "EMPRESA LTDA - CNPJ"
	// "CNPJ da EMPRESA LTDA"

	patterns := []string{
		`[Rr]azão\s+[Ss]ocial:?\s*([A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][A-ZÀÁÂÃÇÉÊÍÓÔÕÚ\s\-\.&]+(?:LTDA|S\.A\.|EIRELI|ME|EPP|CIA))`,
		`CNPJ\s+(?:da|de)?\s*([A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][A-ZÀÁÂÃÇÉÊÍÓÔÕÚ\s\-\.&]+(?:LTDA|S\.A\.|EIRELI|ME|EPP|CIA))`,
		`([A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][A-ZÀÁÂÃÇÉÊÍÓÔÕÚ\s\-\.&]+(?:LTDA|S\.A\.|EIRELI|ME|EPP|CIA))\s*-\s*CNPJ`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) >= 2 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// extractSocios extrai nomes de sócios de um texto
func extractSocios(text string) []string {
	var socios []string

	// Padrões comuns:
	// "Sócios: João Silva, Maria Santos"
	// "Administradores: João Silva"
	// "Sócio Administrador: João Silva"

	patterns := []string{
		`[Ss]ócios?:?\s*([A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+(?:\s+[A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+)+)`,
		`[Aa]dministradores?:?\s*([A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+(?:\s+[A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+)+)`,
		`[Ss]ócio\s+[Aa]dministrador:?\s*([A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+(?:\s+[A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+)+)`,
		`[Pp]roprietário:?\s*([A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+(?:\s+[A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+)+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				nome := strings.TrimSpace(match[1])
				// Valida se parece ser um nome (tem pelo menos 2 palavras, não tem números)
				if isValidName(nome) {
					socios = append(socios, nome)
				}
			}
		}
	}

	// Busca também padrão de lista: "João Silva e Maria Santos"
	reList := regexp.MustCompile(`([A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+(?:\s+[A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+)+)\s+e\s+([A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+(?:\s+[A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+)+)`)
	if matches := reList.FindAllStringSubmatch(text, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) >= 3 {
				if isValidName(match[1]) {
					socios = append(socios, strings.TrimSpace(match[1]))
				}
				if isValidName(match[2]) {
					socios = append(socios, strings.TrimSpace(match[2]))
				}
			}
		}
	}

	// Busca padrão de vírgula: "João Silva, Maria Santos"
	if strings.Contains(text, "sócio") || strings.Contains(text, "administrador") {
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), "sócio") || strings.Contains(strings.ToLower(line), "administrador") {
				parts := strings.Split(line, ",")
				for _, part := range parts {
					// Tenta extrair nome
					reName := regexp.MustCompile(`([A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+(?:\s+[A-ZÀÁÂÃÇÉÊÍÓÔÕÚ][a-zàáâãçéêíóôõú]+)+)`)
					if matches := reName.FindStringSubmatch(part); len(matches) >= 2 {
						nome := strings.TrimSpace(matches[1])
						if isValidName(nome) {
							socios = append(socios, nome)
						}
					}
				}
			}
		}
	}

	return socios
}

// extractTelefonesFromText extrai telefones de um texto
func extractTelefonesFromText(text string) []string {
	var telefones []string

	// Padrões de telefone:
	// (11) 1234-5678
	// (11) 91234-5678
	// 11 1234-5678
	// 1112345678

	patterns := []string{
		`\((\d{2})\)\s*(\d{4,5}[-\s]?\d{4})`,
		`(\d{2})\s+(\d{4,5}[-\s]?\d{4})`,
		`telefone:?\s*(\d{10,11})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			var telefone string
			if len(match) >= 3 {
				// Formato (XX) XXXXX-XXXX
				telefone = fmt.Sprintf("(%s) %s", match[1], match[2])
			} else if len(match) >= 2 {
				telefone = match[1]
			}

			if telefone != "" {
				// Normaliza formato
				telefone = normalizeTelefone(telefone)
				if telefone != "" {
					telefones = append(telefones, telefone)
				}
			}
		}
	}

	return telefones
}

// isValidName verifica se uma string parece ser um nome válido
func isValidName(name string) bool {
	// Deve ter pelo menos 2 palavras
	words := strings.Fields(name)
	if len(words) < 2 {
		return false
	}

	// Não deve ter números
	if regexp.MustCompile(`\d`).MatchString(name) {
		return false
	}

	// Não deve ter palavras muito curtas (< 2 letras) exceto preposições comuns
	preposicoes := map[string]bool{"de": true, "da": true, "do": true, "e": true}
	for _, word := range words {
		if len(word) < 2 && !preposicoes[strings.ToLower(word)] {
			return false
		}
	}

	// Não deve ter palavras em caixa alta completa (exceto siglas de 2-3 letras)
	for _, word := range words {
		if len(word) > 3 && word == strings.ToUpper(word) {
			return false
		}
	}

	return true
}

// normalizeTelefone normaliza formato de telefone
func normalizeTelefone(tel string) string {
	// Remove caracteres não numéricos
	re := regexp.MustCompile(`\D`)
	digits := re.ReplaceAllString(tel, "")

	// Deve ter 10 ou 11 dígitos
	if len(digits) < 10 || len(digits) > 11 {
		return ""
	}

	// Formata: (XX) XXXX-XXXX ou (XX) XXXXX-XXXX
	if len(digits) == 10 {
		return fmt.Sprintf("(%s) %s-%s", digits[0:2], digits[2:6], digits[6:10])
	} else {
		return fmt.Sprintf("(%s) %s-%s", digits[0:2], digits[2:7], digits[7:11])
	}
}

// removeDuplicates remove strings duplicadas mantendo ordem
func removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, item := range slice {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			result = append(result, item)
		}
	}

	return result
}

// BingSearchScraper busca via Bing (alternativa ao DuckDuckGo)
type BingSearchScraper struct{}

func NewBingSearchScraper() *BingSearchScraper {
	return &BingSearchScraper{}
}

func (b *BingSearchScraper) Name() string {
	return "Bing Search"
}

// EnrichFromBing busca dados de sócios via Bing
func EnrichFromBing(ctx context.Context, cnpj *CNPJ) error {
	if cnpj == nil || cnpj.Number == "" {
		return fmt.Errorf("CNPJ inválido")
	}

	searchQuery := fmt.Sprintf("%s sócios administradores", cnpj.Number)
	url := fmt.Sprintf("https://www.bing.com/search?q=%s", searchQuery)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("erro ao criar requisição: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	time.Sleep(1 * time.Second)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("erro na requisição: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("erro ao parsear HTML: %w", err)
	}

	updated := false
	var socios []string
	var telefones []string

	// Extrai de snippets
	doc.Find(".b_caption, .b_snippet").Each(func(i int, s *goquery.Selection) {
		text := s.Text()

		foundSocios := extractSocios(text)
		socios = append(socios, foundSocios...)

		foundTelefones := extractTelefonesFromText(text)
		telefones = append(telefones, foundTelefones...)
	})

	// Atualiza sócios
	if len(socios) > 0 {
		sociosMap := make(map[string]bool)
		for _, s := range cnpj.Socios {
			sociosMap[strings.ToLower(s)] = true
		}

		for _, s := range removeDuplicates(socios) {
			if !sociosMap[strings.ToLower(s)] {
				cnpj.Socios = append(cnpj.Socios, s)
				updated = true
			}
		}
	}

	// Atualiza telefones
	if len(telefones) > 0 {
		telefonesMap := make(map[string]bool)
		for _, t := range cnpj.Telefones {
			telefonesMap[t] = true
		}

		for _, t := range removeDuplicates(telefones) {
			if !telefonesMap[t] {
				cnpj.Telefones = append(cnpj.Telefones, t)
				updated = true
			}
		}
	}

	if !updated {
		return fmt.Errorf("nenhum dado novo encontrado no Bing")
	}

	return nil
}

// BraveSearchScraper busca via Brave Search
type BraveSearchScraper struct{}

func NewBraveSearchScraper() *BraveSearchScraper {
	return &BraveSearchScraper{}
}

func (b *BraveSearchScraper) Name() string {
	return "Brave Search"
}

// EnrichFromBrave busca dados de sócios via Brave Search
func EnrichFromBrave(ctx context.Context, cnpj *CNPJ) error {
	if cnpj == nil || cnpj.Number == "" {
		return fmt.Errorf("CNPJ inválido")
	}

	searchQuery := fmt.Sprintf("%s sócios administradores", cnpj.Number)
	url := fmt.Sprintf("https://search.brave.com/search?q=%s", searchQuery)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("erro ao criar requisição: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	time.Sleep(1 * time.Second)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("erro na requisição: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("erro ao parsear HTML: %w", err)
	}

	updated := false
	var socios []string
	var telefones []string

	// Brave usa classes específicas para snippets
	doc.Find(".snippet, .snippet-description, .snippet-content").Each(func(i int, s *goquery.Selection) {
		text := s.Text()

		foundSocios := extractSocios(text)
		socios = append(socios, foundSocios...)

		foundTelefones := extractTelefonesFromText(text)
		telefones = append(telefones, foundTelefones...)
	})

	// Fallback: busca em divs gerais de resultado
	if len(socios) == 0 && len(telefones) == 0 {
		doc.Find(".result, .search-result").Each(func(i int, s *goquery.Selection) {
			text := s.Text()

			foundSocios := extractSocios(text)
			socios = append(socios, foundSocios...)

			foundTelefones := extractTelefonesFromText(text)
			telefones = append(telefones, foundTelefones...)
		})
	}

	// Atualiza sócios
	if len(socios) > 0 {
		sociosMap := make(map[string]bool)
		for _, s := range cnpj.Socios {
			sociosMap[strings.ToLower(s)] = true
		}

		for _, s := range removeDuplicates(socios) {
			if !sociosMap[strings.ToLower(s)] {
				cnpj.Socios = append(cnpj.Socios, s)
				updated = true
			}
		}
	}

	// Atualiza telefones
	if len(telefones) > 0 {
		telefonesMap := make(map[string]bool)
		for _, t := range cnpj.Telefones {
			telefonesMap[t] = true
		}

		for _, t := range removeDuplicates(telefones) {
			if !telefonesMap[t] {
				cnpj.Telefones = append(cnpj.Telefones, t)
				updated = true
			}
		}
	}

	if !updated {
		return fmt.Errorf("nenhum dado novo encontrado no Brave")
	}

	return nil
}

// YandexSearchScraper busca via Yandex
type YandexSearchScraper struct{}

func NewYandexSearchScraper() *YandexSearchScraper {
	return &YandexSearchScraper{}
}

func (y *YandexSearchScraper) Name() string {
	return "Yandex Search"
}

// EnrichFromYandex busca dados de sócios via Yandex
func EnrichFromYandex(ctx context.Context, cnpj *CNPJ) error {
	if cnpj == nil || cnpj.Number == "" {
		return fmt.Errorf("CNPJ inválido")
	}

	searchQuery := fmt.Sprintf("%s sócios administradores Brasil", cnpj.Number)
	url := fmt.Sprintf("https://yandex.com/search/?text=%s&lr=102", searchQuery) // lr=102 = Brasil

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("erro ao criar requisição: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")

	time.Sleep(1 * time.Second)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("erro na requisição: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("erro ao parsear HTML: %w", err)
	}

	updated := false
	var socios []string
	var telefones []string

	// Yandex usa classes específicas para snippets
	doc.Find(".Organic-Text, .OrganicText, .ExtendedText").Each(func(i int, s *goquery.Selection) {
		text := s.Text()

		foundSocios := extractSocios(text)
		socios = append(socios, foundSocios...)

		foundTelefones := extractTelefonesFromText(text)
		telefones = append(telefones, foundTelefones...)
	})

	// Fallback: busca em títulos e descrições gerais
	if len(socios) == 0 && len(telefones) == 0 {
		doc.Find(".Organic, .serp-item").Each(func(i int, s *goquery.Selection) {
			text := s.Text()

			foundSocios := extractSocios(text)
			socios = append(socios, foundSocios...)

			foundTelefones := extractTelefonesFromText(text)
			telefones = append(telefones, foundTelefones...)
		})
	}

	// Atualiza sócios
	if len(socios) > 0 {
		sociosMap := make(map[string]bool)
		for _, s := range cnpj.Socios {
			sociosMap[strings.ToLower(s)] = true
		}

		for _, s := range removeDuplicates(socios) {
			if !sociosMap[strings.ToLower(s)] {
				cnpj.Socios = append(cnpj.Socios, s)
				updated = true
			}
		}
	}

	// Atualiza telefones
	if len(telefones) > 0 {
		telefonesMap := make(map[string]bool)
		for _, t := range cnpj.Telefones {
			telefonesMap[t] = true
		}

		for _, t := range removeDuplicates(telefones) {
			if !telefonesMap[t] {
				cnpj.Telefones = append(cnpj.Telefones, t)
				updated = true
			}
		}
	}

	if !updated {
		return fmt.Errorf("nenhum dado novo encontrado no Yandex")
	}

	return nil
}
