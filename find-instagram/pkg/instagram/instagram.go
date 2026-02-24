package instagram

import (
	"regexp"
	"strings"
)

// Instagram representa um handle do Instagram
type Instagram struct {
	Handle    string // Ex: dimazzomenswear
	URL       string // Ex: https://instagram.com/dimazzomenswear
	Formatted string // Ex: @dimazzomenswear
}

// NewInstagram cria uma nova instância de Instagram
func NewInstagram(handle string) *Instagram {
	handle = NormalizeHandle(handle)
	if handle == "" {
		return nil
	}

	return &Instagram{
		Handle:    handle,
		URL:       "https://instagram.com/" + handle,
		Formatted: "@" + handle,
	}
}

// NormalizeHandle normaliza um handle do Instagram
func NormalizeHandle(handle string) string {
	// Remove espaços
	handle = strings.TrimSpace(handle)

	// Remove @ se existir
	handle = strings.TrimPrefix(handle, "@")

	// Remove / e URL completa se existir
	handle = strings.TrimPrefix(handle, "https://")
	handle = strings.TrimPrefix(handle, "http://")
	handle = strings.TrimPrefix(handle, "www.")
	handle = strings.TrimPrefix(handle, "instagram.com/")
	handle = strings.TrimPrefix(handle, "instagr.am/")

	// Remove query params e fragments
	if idx := strings.Index(handle, "?"); idx != -1 {
		handle = handle[:idx]
	}
	if idx := strings.Index(handle, "#"); idx != -1 {
		handle = handle[:idx]
	}

	// Remove trailing slash
	handle = strings.TrimSuffix(handle, "/")

	// Valida formato básico
	if !IsValidHandle(handle) {
		return ""
	}

	return handle
}

// IsValidHandle verifica se um handle é válido
func IsValidHandle(handle string) bool {
	if handle == "" {
		return false
	}

	// Instagram usernames:
	// - 1-30 caracteres
	// - Apenas letras, números, underscores e pontos
	// - Não pode ter dois pontos consecutivos
	// - Não pode começar ou terminar com ponto
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9._]){0,28}[a-zA-Z0-9]$`, handle)
	return matched
}

// ExtractHandle extrai um handle do Instagram de um texto
func ExtractHandle(text string) *Instagram {
	handles := ExtractAllHandles(text)
	if len(handles) > 0 {
		return handles[0]
	}
	return nil
}

// ExtractAllHandles extrai todos os handles do Instagram de um texto
func ExtractAllHandles(text string) []*Instagram {
	var handles []*Instagram
	seen := make(map[string]bool)

	// Padrões de regex para encontrar handles
	patterns := []*regexp.Regexp{
		// @username
		regexp.MustCompile(`@([a-zA-Z0-9._]{1,30})`),
		// instagram.com/username
		regexp.MustCompile(`instagram\.com/([a-zA-Z0-9._]{1,30})`),
		// instagr.am/username
		regexp.MustCompile(`instagr\.am/([a-zA-Z0-9._]{1,30})`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				handle := NormalizeHandle(match[1])
				if handle != "" && !seen[handle] {
					instagram := NewInstagram(handle)
					if instagram != nil {
						handles = append(handles, instagram)
						seen[handle] = true
					}
				}
			}
		}
	}

	return handles
}

// NormalizarQuery normaliza uma query de busca
func NormalizarQuery(query string) string {
	query = strings.TrimSpace(query)
	query = strings.ToLower(query)

	// Remove "instagram" redundante da query se já estiver presente
	// mas mantém se for a única palavra
	words := strings.Fields(query)
	if len(words) > 1 {
		var filtered []string
		for _, word := range words {
			if word != "instagram" && word != "ig" {
				filtered = append(filtered, word)
			}
		}
		if len(filtered) > 0 {
			query = strings.Join(filtered, " ")
		}
	}

	return query
}
