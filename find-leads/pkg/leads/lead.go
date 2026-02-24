package leads

import (
	"regexp"
	"sort"
	"strings"
)

// Lead representa um estabelecimento/lead encontrado
type Lead struct {
	Name     string
	Phone    string
	Phone2   string
	Address  string
	City     string
	State    string
	Category string
	Website  string
	Email    string
	CNPJ     string
	Rating   string
	Source   string
}

func (l *Lead) NormalizedName() string {
	return normalizeString(l.Name)
}

func (l *Lead) NormalizedPhone() string {
	re := regexp.MustCompile(`\D`)
	return re.ReplaceAllString(l.Phone, "")
}

func normalizeString(s string) string {
	accents := map[rune]rune{
		'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
		'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
		'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
		'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
		'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
		'ç': 'c', 'ñ': 'n',
		'Á': 'a', 'À': 'a', 'Â': 'a', 'Ã': 'a',
		'É': 'e', 'È': 'e', 'Ê': 'e',
		'Í': 'i', 'Ì': 'i',
		'Ó': 'o', 'Ò': 'o', 'Ô': 'o', 'Õ': 'o',
		'Ú': 'u', 'Ù': 'u', 'Û': 'u',
		'Ç': 'c', 'Ñ': 'n',
	}
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if mapped, ok := accents[r]; ok {
			b.WriteRune(mapped)
		} else {
			b.WriteRune(r)
		}
	}
	result := b.String()
	re := regexp.MustCompile(`[^a-z0-9\s]`)
	result = re.ReplaceAllString(result, " ")
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	return strings.TrimSpace(result)
}

// wordBagKey retorna uma chave canônica para deduplicação por "bag-of-words":
// lowercase sem acentos, palavras ordenadas alfabeticamente.
// Isso captura variações de ordem como "Loja de Roupas XYZ" == "XYZ Loja de Roupas".
func wordBagKey(name string) string {
	normalized := normalizeString(name)
	words := strings.Fields(normalized)
	if len(words) < 2 {
		return "" // nomes de palavra única: sem vantagem em usar word-bag
	}
	sort.Strings(words)
	return strings.Join(words, " ")
}

// Deduplicate remove leads duplicados priorizando os com mais dados
func Deduplicate(leadsList []*Lead) []*Lead {
	byPhone := make(map[string]*Lead)
	byName := make(map[string]*Lead)
	byWordBag := make(map[string]*Lead)
	var result []*Lead

	score := func(l *Lead) int {
		s := 0
		if l.Phone != "" {
			s += 3
		}
		if l.Address != "" {
			s += 2
		}
		if l.CNPJ != "" {
			s += 2
		}
		if l.Website != "" {
			s++
		}
		if l.Email != "" {
			s++
		}
		if l.Category != "" {
			s++
		}
		return s
	}
	_ = score

	merge := func(existing, incoming *Lead) {
		if existing.Phone == "" && incoming.Phone != "" {
			existing.Phone = incoming.Phone
		}
		if existing.Phone2 == "" && incoming.Phone != "" && incoming.Phone != existing.Phone {
			existing.Phone2 = incoming.Phone
		}
		if existing.Address == "" && incoming.Address != "" {
			existing.Address = incoming.Address
		}
		if existing.Website == "" && incoming.Website != "" {
			existing.Website = incoming.Website
		}
		if existing.Email == "" && incoming.Email != "" {
			existing.Email = incoming.Email
		}
		if existing.CNPJ == "" && incoming.CNPJ != "" {
			existing.CNPJ = incoming.CNPJ
		}
		if existing.Category == "" && incoming.Category != "" {
			existing.Category = incoming.Category
		}
		if existing.Rating == "" && incoming.Rating != "" {
			existing.Rating = incoming.Rating
		}
		if !strings.Contains(existing.Source, incoming.Source) {
			existing.Source += "+" + incoming.Source
		}
	}

	for _, lead := range leadsList {
		if lead.Name == "" {
			continue
		}
		phone := lead.NormalizedPhone()
		name := lead.NormalizedName()
		bagKey := wordBagKey(lead.Name)

		if phone != "" && len(phone) >= 8 {
			if existing, ok := byPhone[phone]; ok {
				merge(existing, lead)
				continue
			}
		}
		if name != "" {
			if existing, ok := byName[name]; ok {
				merge(existing, lead)
				if phone != "" && len(phone) >= 8 {
					byPhone[phone] = existing
				}
				continue
			}
		}
		if bagKey != "" {
			if existing, ok := byWordBag[bagKey]; ok {
				merge(existing, lead)
				if phone != "" && len(phone) >= 8 {
					byPhone[phone] = existing
				}
				continue
			}
		}
		result = append(result, lead)
		if phone != "" && len(phone) >= 8 {
			byPhone[phone] = lead
		}
		if name != "" {
			byName[name] = lead
		}
		if bagKey != "" {
			byWordBag[bagKey] = lead
		}
	}
	return result
}
