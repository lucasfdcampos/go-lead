package leads

import (
"context"
"fmt"
"html"
"io"
"net/http"
"regexp"
"strings"
"sync"
"time"
)

const (
solutudoBase       = "https://www.solutudo.com.br/empresas"
solutudoTimeout    = 12 * time.Second
solutudoMaxWorkers = 4 // goroutines concorrentes
)

// SolutudoScraper busca leads no Solutudo via regex em JSON-LD embutido no HTML.
// Solutudo embute objetos JSON-LD "@type":"LocalBusiness" diretamente no HTML.
type SolutudoScraper struct {
client *http.Client
}

func NewSolutudoScraper() *SolutudoScraper {
return &SolutudoScraper{
client: &http.Client{Timeout: solutudoTimeout},
}
}

func (s *SolutudoScraper) Name() string { return "Solutudo" }

func (s *SolutudoScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
city, state := ParseLocation(location)
if city == "" || state == "" {
return nil, fmt.Errorf("solutudo: localização inválida %q", location)
}

stateSlug := strings.ToLower(state)
citySlug := CitySlug(city)
querySlug := QuerySlug(query)

urls := solutudoBuildURLs(solutudoBase, citySlug, stateSlug, querySlug)

type pageResult struct {
names []string
}
pageResults := make([]pageResult, len(urls))
sem := make(chan struct{}, solutudoMaxWorkers)
var wg sync.WaitGroup

for i, u := range urls {
wg.Add(1)
go func(idx int, rawURL string) {
defer wg.Done()
sem <- struct{}{}
defer func() { <-sem }()

pageCtx, cancel := context.WithTimeout(ctx, solutudoTimeout)
names, _ := solutudoFetchPage(s.client, pageCtx, rawURL)
cancel()
pageResults[idx] = pageResult{names: names}
}(i, u)
}
wg.Wait()

// Merge deduplicando por lowercase
seen := make(map[string]bool)
var leads []*Lead
for _, r := range pageResults {
for _, name := range r.names {
key := strings.ToLower(name)
if !seen[key] {
seen[key] = true
leads = append(leads, &Lead{
Name:   name,
City:   city,
State:  state,
Source: "Solutudo",
})
}
}
}

return leads, nil
}

// solutudoFetchPage faz o fetch de uma URL e extrai nomes via JSON-LD regex.
func solutudoFetchPage(client *http.Client, ctx context.Context, rawURL string) ([]string, error) {
req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
if err != nil {
return nil, err
}
req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:124.0) Gecko/20100101 Firefox/124.0")
req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")
req.Header.Set("Accept", "text/html,application/xhtml+xml")

resp, err := client.Do(req)
if err != nil {
return nil, err
}
defer resp.Body.Close()

if resp.StatusCode == http.StatusNotFound {
return nil, nil
}
if resp.StatusCode != http.StatusOK {
return nil, fmt.Errorf("solutudo: HTTP %d para %s", resp.StatusCode, rawURL)
}

body, err := io.ReadAll(resp.Body)
if err != nil {
return nil, err
}

return solutudoParseNames(string(body)), nil
}

// ─── Parsing ─────────────────────────────────────────────────────────────────

// reSolutudoName extrai nomes de empresas de objetos JSON-LD LocalBusiness.
var reSolutudoName = regexp.MustCompile(`"@type"\s*:\s*"LocalBusiness"\s*,\s*"name"\s*:\s*"([^"]+)"`)

// reSolutudoNameAlt é fallback para variações de ordem no JSON-LD.
var reSolutudoNameAlt = regexp.MustCompile(`"name"\s*:\s*"([^"]+)"\s*,\s*"@type"\s*:\s*"LocalBusiness"`)

func solutudoParseNames(body string) []string {
seen := make(map[string]bool)
var names []string

addName := func(raw string) {
name := html.UnescapeString(strings.TrimSpace(raw))
if len(name) < 3 {
return
}
lower := strings.ToLower(name)
if !seen[lower] {
seen[lower] = true
names = append(names, name)
}
}

for _, m := range reSolutudoName.FindAllStringSubmatch(body, 500) {
addName(m[1])
}
for _, m := range reSolutudoNameAlt.FindAllStringSubmatch(body, 500) {
addName(m[1])
}

return names
}

// ─── URL building ─────────────────────────────────────────────────────────────

// solutudoCategoryMap mapeia queries para slugs de categoria do Solutudo.
// URL: https://www.solutudo.com.br/empresas/{estado}/{cidade}/{categoria}/
var solutudoCategoryMap = map[string][]string{
"loja de roupas": {"confeccoes", "roupas-e-acessorios"},
"roupa":          {"confeccoes", "roupas-e-acessorios"},
"roupas":         {"confeccoes", "roupas-e-acessorios"},
"vestuario":      {"confeccoes", "roupas-e-acessorios"},
"vestuário":      {"confeccoes", "roupas-e-acessorios"},
"boutique":       {"roupas-e-acessorios"},
"moda":           {"confeccoes", "roupas-e-acessorios"},

"restaurante":  {"restaurantes", "churrascarias", "lanchonetes"},
"restaurantes": {"restaurantes", "churrascarias"},
"lanchonete":   {"lanchonetes", "restaurantes"},
"pizzaria":     {"pizzarias", "lanchonetes"},
"padaria":      {"padarias"},
"bar":          {"bares"},
"churrascaria": {"churrascarias", "restaurantes"},

"farmacia":  {"farmacias"},
"farmácia":  {"farmacias"},
"academia":  {"academias"},
"autopecas": {"autopecas"},
"autopeças": {"autopecas"},
"calcado":   {"calcados"},
"calçado":   {"calcados"},
"sapato":    {"calcados"},

"salao de beleza": {"saloes-de-beleza"},
"salão de beleza": {"saloes-de-beleza"},
"barbearia":       {"barbearias"},
"cabeleireiro":    {"cabeleireiros"},
"manicure":        {"manicures-e-pedicures"},

"mecanica":            {"mecanicas", "autopecas"},
"mecânica":            {"mecanicas", "autopecas"},
"oficina":             {"mecanicas"},
"funilaria":           {"funilarias"},
"vidracaria":          {"vidracarias"},
"eletrica automotiva": {"eletrica-automotiva"},
"borracharia":         {"borracharias"},
"lava jato":           {"lava-jatos"},
"concessionaria":      {"concessionarias"},
"concessionária":      {"concessionarias"},

"advocacia":     {"advocacias", "escritorios-de-advocacia"},
"advogado":      {"advocacias"},
"contabilidade": {"contabilidades"},
"contador":      {"contabilidades"},
"imobiliaria":   {"imobiliarias"},
"imobiliária":   {"imobiliarias"},
"despachante":   {"despachantes"},
"seguradora":    {"seguradoras"},
"cartorio":      {"cartorios"},
"cartório":      {"cartorios"},
"consultoria":   {"consultorias"},

"construcao":             {"construcoes", "materiais-de-construcao"},
"construção":             {"construcoes", "materiais-de-construcao"},
"construtora":            {"construtoras"},
"eletricista":            {"eletricistas"},
"encanador":              {"encanadores"},
"pintura":                {"pinturas"},
"marcenaria":             {"marcenarias"},
"serralheria":            {"serralherias"},
"dedetizacao":            {"dedetizacoes"},
"dedetização":            {"dedetizacoes"},
"material de construcao": {"materiais-de-construcao"},
"material de construção": {"materiais-de-construcao"},

"escola":       {"escolas"},
"colegio":      {"colegios"},
"colégio":      {"colegios"},
"faculdade":    {"faculdades"},
"universidade": {"universidades"},
"idiomas":      {"cursos-de-idiomas"},
"curso":        {"cursos"},
"autoescola":   {"autoescolas"},

"clinica":      {"clinicas", "clinicas-medicas"},
"clínica":      {"clinicas", "clinicas-medicas"},
"laboratorio":  {"laboratorios"},
"laboratório":  {"laboratorios"},
"fisioterapia": {"fisioterapias"},
"nutricao":     {"nutricoes"},
"nutrição":     {"nutricoes"},
"psicologia":   {"psicologias"},
"ortopedia":    {"ortopedias"},
"dermatologia": {"dermatologias"},
"oftalmologia": {"oftalmologias"},

"sorveteria":   {"sorveteiras", "lanchonetes"},
"doceria":      {"doceiras", "confeitarias"},
"confeitaria":  {"confeitarias"},
"hortifruti":   {"hortifrutis"},
"mercearia":    {"mercearias"},
"acougue":      {"acougues"},
"açougue":      {"acougues"},
"cafeteria":    {"cafeterias", "lanchonetes"},
"hamburgueria": {"hamburguerias", "lanchonetes"},
"marmitaria":   {"marmitarias", "restaurantes"},

"supermercado": {"supermercados"},
"papelaria":    {"papelarias"},
"livraria":     {"livrarias"},
"informatica":  {"informaticas"},
"informática":  {"informaticas"},
"celular":      {"celulares"},
"optica":       {"oticas"},
"ótica":        {"oticas"},
"floricultura": {"floriculturas"},
"brinquedos":   {"brinquedos"},

"hotel":      {"hoteis"},
"pousada":    {"pousadas"},
"buffet":     {"buffets"},
"fotografia": {"fotografias", "fotografos"},
}

func solutudoBuildURLs(base, citySlug, stateSlug, querySlug string) []string {
query := strings.ReplaceAll(querySlug, "-", " ")
slugs, ok := solutudoCategoryMap[query]
if !ok {
// Fallback: tenta pluralizar o slug
slugs = []string{querySlug + "s", querySlug}
}

var urls []string
for _, slug := range slugs {
urls = append(urls, fmt.Sprintf("%s/%s/%s/%s/", base, stateSlug, citySlug, slug))
}
return urls
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func normalizePhone(phone string) string {
re := regexp.MustCompile(`\D`)
digits := re.ReplaceAllString(phone, "")
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
return ""
}
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
