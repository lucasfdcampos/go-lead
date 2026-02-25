// Package filter provides post-discovery, pre-response lead filtering.
//
// Three passes are available:
//  1. ByNameRelevance – always-on; uses business name keywords to discard
//     leads that clearly belong to a different category than the query.
//  2. ByLocation      – post-CNPJ; discards leads whose enriched Municipio
//     doesn't match the requested city.
//  3. ByCategory      – post-CNPJ; discards leads whose CNAE code is not
//     in the set of compatible codes for the query.
package filter

import (
	"strings"

	"github.com/lucasfdcampos/lead-api/internal/domain"
)

// ─── nameKeywords ─────────────────────────────────────────────────────────────

// nameKeywords maps lowercase keywords that commonly appear in Brazilian
// business names to the top-level CNAE prefix(es) they indicate.
// Used in ByNameRelevance to classify a lead before CNPJ lookup.
var nameKeywords = map[string][]string{
	// Alimentação
	"restaurante":  {"5611"},
	"churrascaria": {"5611"},
	"lanchonete":   {"5611"},
	"pizzaria":     {"5611"},
	"hamburgueria": {"5611"},
	"padaria":      {"1091", "4721"},
	"confeitaria":  {"1091"},
	"sorveteria":   {"1053", "5611"},
	"açougue":      {"4722"},
	"abatedouro":   {"1012", "1013"},
	"frigorífico":  {"1013"},
	"peixaria":     {"4723"},
	"hortifruti":   {"4724"},
	"bar ":         {"5611"}, // trailing space avoids matching "barbearia"
	"boteco":       {"5611"},
	"lancheria":    {"5611"},
	"rotisseria":   {"4721"},
	"mercearia":    {"4712"},
	// "empório" omitted – too ambiguous (Empório do Jeans vs Empório do Café)
	"supermercado": {"4711"},
	// Saúde
	"farmácia":    {"4771"},
	"drogaria":    {"4771"},
	"clínica":     {"8630"},
	"hospital":    {"8610"},
	"laboratório": {"8640"},
	"dentista":    {"8630"},
	"odontologia": {"8630"},
	"veterinári":  {"7500"}, // veterinário / veterinária
	"pet shop":    {"7500", "4789"},
	"petshop":     {"7500", "4789"},
	// Automóvel / Transporte
	"caminhões":      {"4511", "4512"},
	"caminhao":       {"4511"},
	"veículos":       {"4511"},
	"automóveis":     {"4511"},
	"concessionár":   {"4511"},
	"oficina":        {"4520"},
	"mecânica":       {"4520"},
	"borracharia":    {"4530"},
	"transportadora": {"4930"},
	"motoboy":        {"5320"},
	"locadora de":    {"7711"},
	// Beleza
	"salão":      {"9602"},
	"barbearia":  {"9602"},
	"estética":   {"9602"},
	"cabeleirei": {"9602"},
	// Construção
	"construção":              {"4120"},
	"construtora":             {"4120"},
	"madeireira":              {"1610"},
	"serraria":                {"1610"},
	"marmoraria":              {"2391"},
	"materiais de construção": {"4744"},
	// Hospedagem
	"hotel":   {"5510"},
	"pousada": {"5510"},
	"motel":   {"5510"},
	"hostel":  {"5510"},
	// Impressão / Gráfica
	"gráfica":    {"1811", "1812"},
	"gráficas":   {"1811", "1812"},
	"tipografia": {"1811"},
	// Educação
	"escola":       {"8511", "8512", "8513"},
	"colégio":      {"8512"},
	"faculdade":    {"8530"},
	"universidade": {"8530"},
	// Agro / Campo
	"agropecuária": {"4612", "4623"},
	"fazenda":      {"0111"},
	"granja":       {"0155"},
	// Logística
	"armazém":  {"5211"},
	"depósito": {"5211"},
}

// ─── ByNameRelevance ──────────────────────────────────────────────────────────

// ByNameRelevance filters leads whose business names strongly indicate a
// category incompatible with the search query.
//
// Only fires when the query has a known CNAE mapping (via cnae.cnaePrefixMap).
// When no mapping exists for the query, all leads are kept.
//
// Returns (kept leads, number discarded).
func ByNameRelevance(leads []domain.Lead, query string) ([]domain.Lead, int) {
	queryPrefixes := expectedPrefixes(query)
	if len(queryPrefixes) == 0 {
		// Unknown query type — keep everything.
		return leads, 0
	}

	kept := make([]domain.Lead, 0, len(leads))
	discarded := 0

	for _, l := range leads {
		detected := detectCNAEFromName(l.Name)
		if len(detected) > 0 && !prefixesOverlap(queryPrefixes, detected) {
			// The name strongly indicates an incompatible category.
			discarded++
			continue
		}
		kept = append(kept, l)
	}
	return kept, discarded
}

// ─── ByLocation ───────────────────────────────────────────────────────────────

// ByLocation removes leads whose enriched Municipio is populated and doesn't
// match the requested city (case-insensitive, accent-insensitive comparison).
//
// Leads without a Municipio (not yet enriched) are always kept.
// Returns (kept leads, number discarded).
func ByLocation(leads []domain.Lead, city, state string) ([]domain.Lead, int) {
	if city == "" {
		return leads, 0
	}
	wantCity := normalize(city)
	wantUF := strings.ToUpper(strings.TrimSpace(state))

	kept := make([]domain.Lead, 0, len(leads))
	discarded := 0

	for _, l := range leads {
		if l.Municipio == "" {
			// No city data yet – keep it.
			kept = append(kept, l)
			continue
		}
		gotCity := normalize(l.Municipio)
		gotUF := strings.ToUpper(strings.TrimSpace(l.UF))

		cityMatch := gotCity == wantCity
		// If we have both UF values, also check state. Otherwise just city.
		ufMatch := wantUF == "" || gotUF == "" || gotUF == wantUF

		if cityMatch && ufMatch {
			kept = append(kept, l)
		} else {
			discarded++
		}
	}
	return kept, discarded
}

// ─── ByCategory ───────────────────────────────────────────────────────────────

// ByCategory removes leads whose enriched CNAE code is populated and not in
// the compatible set.
//
// Leads without a CNAE code (not yet enriched, or CNPJ not found) are kept.
// Returns (kept leads, number discarded).
func ByCategory(leads []domain.Lead, compatibleCodes []string) ([]domain.Lead, int) {
	if len(compatibleCodes) == 0 {
		return leads, 0
	}
	codeSet := make(map[string]bool, len(compatibleCodes))
	for _, c := range compatibleCodes {
		codeSet[strings.TrimSpace(c)] = true
	}

	kept := make([]domain.Lead, 0, len(leads))
	discarded := 0

	for _, l := range leads {
		if l.CNPJ == "" {
			// CNPJ enrichment didn't run or failed – keep.
			kept = append(kept, l)
			continue
		}
		// Extract the raw CNAE code from the lead (stored during enrichment as
		// the CNAECode field). We infer it from CNAEMatch for now — if explicitly
		// stored we'd use it directly. For now use the cnaePrefixMap-style check:
		// We keep leads that have CNAEMatch == nil or CNAEMatch == true.
		if l.CNAEMatch == nil || *l.CNAEMatch {
			kept = append(kept, l)
		} else {
			discarded++
		}
	}
	return kept, discarded
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// expectedPrefixes returns the CNAE prefix list for the search query,
// reusing the same lookup logic as cnae.IsCompatible without importing it
// (to avoid a circular dependency).
var cnaePrefixMap = map[string][]string{
	// Vestuário / Moda
	"loja de roupas": {"4781", "1412", "1411", "4642", "4644"},
	"moda feminina":  {"4781", "1412"},
	"moda masculina": {"4781", "1411"},
	"boutique":       {"4781", "1412", "1411"},
	"confecção":      {"1412", "1411", "1413"},
	"brechó":         {"4781"},
	"multimarcas":    {"4781"},
	// Alimentação
	"restaurante":  {"5611", "5612"},
	"lanchonete":   {"5611"},
	"pizzaria":     {"5611"},
	"padaria":      {"1091", "4721"},
	"bar":          {"5611", "5612"},
	"cafeteria":    {"5612"},
	"sorveteria":   {"5611", "1053"},
	"açougue":      {"4722"},
	"mercearia":    {"4712"},
	"supermercado": {"4711"},
	"mercado":      {"4711", "4712"},
	"hortifruti":   {"4724"},
	"quitanda":     {"4724"},
	"delicatessen": {"4721"},
	"empório":      {"4721"},
	// Saúde & Beleza
	"farmácia":        {"4771"},
	"drogaria":        {"4771"},
	"clínica":         {"8630", "8621", "8622"},
	"clínica médica":  {"8630"},
	"dentista":        {"8630"},
	"psicólogo":       {"8630"},
	"academia":        {"9313"},
	"salão de beleza": {"9602"},
	"barbearia":       {"9602"},
	"estética":        {"9602"},
	"spa":             {"9609"},
	"óptica":          {"4774"},
	"veterinário":     {"7500"},
	"pet shop":        {"4789", "7500"},
	// Automotivo
	"oficina":        {"4520"},
	"mecânica":       {"4520"},
	"funilaria":      {"4520"},
	"borracharia":    {"4530"},
	"auto peças":     {"4541", "4542"},
	"autopeças":      {"4541", "4542"},
	"lavagem":        {"4520"},
	"estacionamento": {"5223"},
	"concessionária": {"4511"},
	"locadora":       {"7711"},
	// Construção & Casa
	"construção":              {"4120", "4399"},
	"materiais de construção": {"4744"},
	"ferragens":               {"4744"},
	"elétrica":                {"4321", "4742"},
	"encanamento":             {"4322"},
	"pintura":                 {"4330"},
	"marcenaria":              {"1610", "1622"},
	"móveis":                  {"4754", "3101", "3102", "3103"},
	"decoração":               {"4759", "7490"},
	"arquitetura":             {"7111"},
	"engenharia":              {"7112"},
	"imobiliária":             {"6811", "6821"},
	"condomínio":              {"8110"},
	// Educação
	"escola":            {"8511", "8512", "8513"},
	"creche":            {"8511"},
	"faculdade":         {"8530"},
	"universidade":      {"8530"},
	"curso":             {"8599"},
	"escola de idiomas": {"8599"},
	"escola de música":  {"8599"},
	// Tecnologia & Serviços
	"ti":               {"6201", "6202", "6209"},
	"software":         {"6201"},
	"informática":      {"4751", "9521"},
	"internet":         {"6110", "6120"},
	"telecomunicações": {"6110"},
	"consultoria":      {"7020", "6920"},
	"contabilidade":    {"6920"},
	"advocacia":        {"6911"},
	"segurança":        {"8011", "8012"},
	// Turismo & Lazer
	"hotel":               {"5510"},
	"pousada":             {"5510"},
	"agência de viagens":  {"7911", "7912"},
	"academia de dança":   {"9313"},
	"academia de natação": {"9313"},
	"cinema":              {"5914"},
	"teatro":              {"9001"},
	"quadra esportiva":    {"9313"},
	// Logística & Transporte
	"transportadora": {"4930", "4921", "4922"},
	"motoboy":        {"5320"},
	"courier":        {"5310", "5320"},
	"armazém":        {"5211"},
	"logística":      {"5229"},
}

func expectedPrefixes(query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if p, ok := cnaePrefixMap[q]; ok {
		return p
	}
	for key, prefixes := range cnaePrefixMap {
		if strings.Contains(q, key) || strings.Contains(key, q) {
			return prefixes
		}
	}
	return nil
}

// detectCNAEFromName returns CNAE prefixes inferred from keywords in the
// business name. Returns nil when no keyword matched.
func detectCNAEFromName(name string) []string {
	lower := strings.ToLower(name)
	for kw, prefixes := range nameKeywords {
		if strings.Contains(lower, kw) {
			return prefixes
		}
	}
	return nil
}

// prefixesOverlap returns true when any prefix in `a` is a prefix of any
// prefix in `b` (or vice-versa) — a top-level category match.
func prefixesOverlap(a, b []string) bool {
	for _, pa := range a {
		for _, pb := range b {
			short := pa
			long := pb
			if len(pb) < len(pa) {
				short, long = pb, pa
			}
			if strings.HasPrefix(long, short) {
				return true
			}
		}
	}
	return false
}

// normalize lowercases and removes common accents for city comparison.
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c", "ñ", "n",
	)
	return replacer.Replace(s)
}
