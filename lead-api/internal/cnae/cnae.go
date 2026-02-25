// Package cnae provides CNAE validation against search queries.
// CNAE (Classificação Nacional de Atividades Econômicas) is the Brazilian
// business activity classification code returned by BrasilAPI.
package cnae

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// cnaePrefixMap maps query keywords (lowercase) to expected CNAE code prefixes.
// A lead is considered a CNAE match when its CNAE code starts with any of the listed prefixes.
// Returning an empty slice means "any CNAE is acceptable for this query".
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
	// Outros varejo
	"papelaria":             {"4761"},
	"livraria":              {"4761"},
	"floricultura":          {"4789"},
	"joalheria":             {"4783"},
	"relojoaria":            {"4783"},
	"brinquedos":            {"4763"},
	"eletrodomésticos":      {"4753"},
	"eletrônicos":           {"4752", "4753"},
	"celular":               {"4752"},
	"instrumentos musicais": {"4756"},
	"artigos religiosos":    {"4789"},
}

// StaticCompatibleCodes returns the CNAE code prefixes from the hardcoded map
// for the given query. Returns nil when the query is unknown.
// (Codes are prefixes, not full codes — callers may use them for prefix matching.)
func StaticCompatibleCodes(query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if p, ok := cnaePrefixMap[q]; ok {
		out := make([]string, len(p))
		copy(out, p)
		return out
	}
	for key, p := range cnaePrefixMap {
		if strings.Contains(q, key) || strings.Contains(key, q) {
			out := make([]string, len(p))
			copy(out, p)
			return out
		}
	}
	return nil
}

// IsCompatible reports whether a business's CNAE code is compatible with the
// search query. Returns true when no mapping exists for the query (conservative —
// we don't want to discard leads with unknown query types).
func IsCompatible(query, cnaeCode string) bool {
	if cnaeCode == "" {
		return true // no CNAE to check
	}
	q := strings.ToLower(strings.TrimSpace(query))

	// Direct key lookup
	if prefixes, ok := cnaePrefixMap[q]; ok {
		return matchesPrefixes(cnaeCode, prefixes)
	}

	// Partial key lookup – check if any map key is contained in the query
	for key, prefixes := range cnaePrefixMap {
		if strings.Contains(q, key) || strings.Contains(key, q) {
			return matchesPrefixes(cnaeCode, prefixes)
		}
	}

	// No match found – be permissive
	return true
}

func matchesPrefixes(cnaeCode string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(cnaeCode, p) {
			return true
		}
	}
	return false
}

// QueryCompatibleCodes queries the leadfinder MongoDB database for CNAE codes
// whose description matches the search query keywords.
// It supplements the static cnaePrefixMap with live data from MongoDB.
// Returns nil and no error when MongoDB is unavailable.
func QueryCompatibleCodes(ctx context.Context, query string, mc *mongo.Client) []string {
	if mc == nil {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	// Split the query into individual words as search terms, skipping short stop words.
	stopwords := map[string]bool{"de": true, "do": true, "da": true, "e": true, "em": true, "a": true, "o": true}
	var keywords []string
	for _, word := range strings.Fields(q) {
		if len(word) >= 3 && !stopwords[word] {
			keywords = append(keywords, word)
		}
	}
	if len(keywords) == 0 {
		return nil
	}

	ors := make(bson.A, 0, len(keywords))
	for _, kw := range keywords {
		ors = append(ors, bson.M{"descricao": bson.M{"$regex": kw, "$options": "i"}})
	}

	coll := mc.Database("leadfinder").Collection("cnaes")
	cursor, err := coll.Find(ctx, bson.M{"$or": ors}, options.Find().SetProjection(bson.M{"codigo": 1}))
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	type cnaedoc struct {
		Codigo string `bson:"codigo"`
	}
	var codes []string
	for cursor.Next(ctx) {
		var doc cnaedoc
		if err := cursor.Decode(&doc); err == nil && doc.Codigo != "" {
			codes = append(codes, doc.Codigo)
		}
	}
	return codes
}
