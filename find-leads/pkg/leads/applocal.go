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
	appLocalBase       = "https://applocal.com.br/empresas"
	appLocalTimeout    = 12 * time.Second
	appLocalMaxPages   = 5 // páginas por subcategoria (~36 por página)
	appLocalMaxWorkers = 8 // goroutines concorrentes
)

// AppLocalScraper busca leads no AppLocal via regex em HTML bruto.
// Estratégia: múltiplas subcategorias × múltiplas páginas em paralelo.
type AppLocalScraper struct {
	client *http.Client
}

func NewAppLocalScraper() *AppLocalScraper {
	return &AppLocalScraper{
		client: &http.Client{Timeout: appLocalTimeout},
	}
}

func (a *AppLocalScraper) Name() string { return "AppLocal" }

func (a *AppLocalScraper) Search(ctx context.Context, query, location string) ([]*Lead, error) {
	city, state := ParseLocation(location)
	if city == "" || state == "" {
		return nil, fmt.Errorf("applocal: localização inválida %q", location)
	}

	citySlug := CitySlug(city)
	stateSlug := strings.ToLower(state)
	querySlug := QuerySlug(query)

	urls := appLocalBuildAllURLs(appLocalBase, citySlug, stateSlug, querySlug, appLocalMaxPages)

	type pageResult struct {
		names []string
	}
	pageResults := make([]pageResult, len(urls))
	sem := make(chan struct{}, appLocalMaxWorkers)
	var wg sync.WaitGroup

	for i, u := range urls {
		wg.Add(1)
		go func(idx int, rawURL string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			pageCtx, cancel := context.WithTimeout(ctx, appLocalTimeout)
			names, _ := appLocalFetchPage(a.client, pageCtx, rawURL)
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
					Source: "AppLocal",
				})
			}
		}
	}

	return leads, nil
}

// appLocalFetchPage faz o fetch de uma URL e extrai nomes via regex.
func appLocalFetchPage(client *http.Client, ctx context.Context, rawURL string) ([]string, error) {
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
		return nil, nil // subcategoria inexistente — ignora
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("applocal: HTTP %d para %s", resp.StatusCode, rawURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return appLocalParseTitles(string(body)), nil
}

// ─── Parsing ─────────────────────────────────────────────────────────────────

// reAppLocalTitle captura nomes de empresas nos atributos title= dos cards.
// Os primeiros ~40 KB são CSS; o conteúdo real começa depois.
var reAppLocalTitle = regexp.MustCompile(`title="([A-ZÁÉÍÓÚÀÂÃÊÔÕÜÇ][^"]{4,80})"`)

// appLocalUITitles são title= de navegação/UI, não empresas.
var appLocalUITitles = map[string]bool{
	"termos de uso":                     true,
	"página inicial":                    true,
	"politica de privacidade":           true,
	"política de privacidade":           true,
	"sobre o applocal":                  true,
	"realizar uma nova pesquisa":        true,
	"cadastre sua empresa":              true,
	"applocal":                          true,
	"mapa":                              true,
	"whatsapp":                          true,
	"facebook":                          true,
	"instagram":                         true,
	"contato com o applocal":            true,
	"encontre empresas por cidade":      true,
	"cadastro gratis de empresa":        true,
	"cadastro grátis de empresa":        true,
	"solicitar exclusao de uma empresa": true,
	"solicitar exclusão de uma empresa": true,
	"concordar e fechar":                true,
	"ver mais empresas":                 true,
	"proxima pagina":                    true,
	"próxima página":                    true,
}

func appLocalParseTitles(body string) []string {
	// Pula o bloco CSS inline (~40 KB) — os cards de empresa ficam depois
	const cssOffset = 40_000
	content := body
	if len(body) > cssOffset {
		content = body[cssOffset:]
	}

	seen := make(map[string]bool)
	var names []string

	for _, m := range reAppLocalTitle.FindAllStringSubmatch(content, 500) {
		name := html.UnescapeString(strings.TrimSpace(m[1]))
		lower := strings.ToLower(name)
		if appLocalUITitles[lower] {
			continue
		}
		if !seen[lower] {
			seen[lower] = true
			names = append(names, name)
		}
	}
	return names
}

// ─── URL building ─────────────────────────────────────────────────────────────

// appLocalCategoryMap mapeia queries para pares [categoria, subcategoria] do AppLocal.
var appLocalCategoryMap = map[string][][2]string{
	"loja de roupas": {
		{"moda-e-vestuario", "loja-de-roupas"},
		{"moda-e-vestuario", "confeccao"},
		{"moda-e-vestuario", "boutique"},
		{"moda-e-vestuario", "brecho"},
		{"moda-e-vestuario", "vestuario"},
		{"moda-e-vestuario", "moda"},
	},
	"roupas":    {{"moda-e-vestuario", "loja-de-roupas"}, {"moda-e-vestuario", "confeccao"}, {"moda-e-vestuario", "moda"}},
	"roupa":     {{"moda-e-vestuario", "loja-de-roupas"}, {"moda-e-vestuario", "confeccao"}},
	"vestuário": {{"moda-e-vestuario", "vestuario"}, {"moda-e-vestuario", "loja-de-roupas"}},
	"vestuario": {{"moda-e-vestuario", "vestuario"}, {"moda-e-vestuario", "loja-de-roupas"}},
	"moda":      {{"moda-e-vestuario", "moda"}, {"moda-e-vestuario", "loja-de-roupas"}},
	"boutique":  {{"moda-e-vestuario", "boutique"}},
	"brechó":    {{"moda-e-vestuario", "brecho"}},
	"brecho":    {{"moda-e-vestuario", "brecho"}},

	"calcado":  {{"moda-e-vestuario", "calcado"}, {"moda-e-vestuario", "sapato"}},
	"calçado":  {{"moda-e-vestuario", "calcado"}, {"moda-e-vestuario", "sapato"}},
	"sapato":   {{"moda-e-vestuario", "sapato"}},
	"sapatos":  {{"moda-e-vestuario", "sapato"}},
	"calcados": {{"moda-e-vestuario", "calcado"}},
	"calçados": {{"moda-e-vestuario", "calcado"}},

	"restaurante":  {{"alimentacao", "restaurante"}},
	"restaurantes": {{"alimentacao", "restaurante"}},
	"lanchonete":   {{"alimentacao", "lanchonete"}},
	"pizzaria":     {{"alimentacao", "pizzaria"}, {"alimentacao", "lanchonete"}},
	"padaria":      {{"alimentacao", "padaria"}},
	"bar":          {{"alimentacao", "bar"}},
	"churrascaria": {{"alimentacao", "churrascaria"}, {"alimentacao", "restaurante"}},

	"farmacia": {{"saude", "farmacia"}},
	"farmácia": {{"saude", "farmacia"}},
	"academia": {{"esportes", "academia"}},

	"cabeleireiro":    {{"beleza", "cabeleireiro"}},
	"salao de beleza": {{"beleza", "salao-de-beleza"}, {"beleza", "cabeleireiro"}},
	"salão de beleza": {{"beleza", "salao-de-beleza"}, {"beleza", "cabeleireiro"}},
	"barbearia":       {{"beleza", "barbearia"}},
	"manicure":        {{"beleza", "manicure"}},

	"autopecas":   {{"automoveis", "autopecas"}},
	"autopeças":   {{"automoveis", "autopecas"}},
	"borracharia": {{"automoveis", "borracharia"}},
	"lavagem":     {{"automoveis", "lavagem"}},

	"supermercado": {{"comercio", "supermercado"}},
	"mercado":      {{"comercio", "mercado"}, {"comercio", "supermercado"}},

	"petshop":     {{"animais", "petshop"}},
	"pet shop":    {{"animais", "petshop"}},
	"veterinario": {{"animais", "veterinario"}},
	"veterinário": {{"animais", "veterinario"}},

	"creche": {{"educacao", "creche"}},
	"hotel":  {{"turismo", "hotel"}},

	"movel":                  {{"casa-e-jardim", "movel"}},
	"móvel":                  {{"casa-e-jardim", "movel"}},
	"moveis":                 {{"casa-e-jardim", "movel"}},
	"móveis":                 {{"casa-e-jardim", "movel"}},
	"eletrodomestico":        {{"casa-e-jardim", "eletrodomestico"}},
	"eletrodoméstico":        {{"casa-e-jardim", "eletrodomestico"}},
	"material de construcao": {{"construcao", "material-de-construcao"}},
	"material de construção": {{"construcao", "material-de-construcao"}},

	"mecanica":            {{"automoveis", "mecanica"}, {"automoveis", "autopecas"}},
	"mecânica":            {{"automoveis", "mecanica"}, {"automoveis", "autopecas"}},
	"oficina":             {{"automoveis", "mecanica"}},
	"funilaria":           {{"automoveis", "funilaria"}, {"automoveis", "mecanica"}},
	"vidracaria":          {{"automoveis", "vidracaria"}, {"construcao", "vidracaria"}},
	"eletrica automotiva": {{"automoveis", "eletrica-automotiva"}, {"automoveis", "mecanica"}},
	"elétrica automotiva": {{"automoveis", "eletrica-automotiva"}, {"automoveis", "mecanica"}},
	"lava jato":           {{"automoveis", "lava-jato"}, {"automoveis", "lavagem"}},
	"concessionaria":      {{"automoveis", "concessionaria"}},
	"concessionária":      {{"automoveis", "concessionaria"}},

	"construcao":        {{"construcao", "material-de-construcao"}, {"construcao", "construtora"}},
	"construção":        {{"construcao", "material-de-construcao"}, {"construcao", "construtora"}},
	"construtora":       {{"construcao", "construtora"}},
	"eletricista":       {{"construcao", "eletricista"}, {"servicos", "eletricista"}},
	"encanador":         {{"construcao", "encanador"}, {"servicos", "encanador"}},
	"pintura":           {{"construcao", "pintura"}, {"servicos", "pintura"}},
	"marcenaria":        {{"construcao", "marcenaria"}, {"casa-e-jardim", "marcenaria"}},
	"serralheria":       {{"construcao", "serralheria"}},
	"marmoraria":        {{"construcao", "marmoraria"}},
	"dedetizacao":       {{"servicos", "dedetizacao"}},
	"dedetização":       {{"servicos", "dedetizacao"}},
	"impermeabilizacao": {{"construcao", "impermeabilizacao"}},
	"impermeabilização": {{"construcao", "impermeabilizacao"}},

	"advocacia":     {{"servicos", "advocacia"}, {"servicos", "escritorio-de-advocacia"}},
	"advogado":      {{"servicos", "advocacia"}},
	"contabilidade": {{"servicos", "contabilidade"}, {"financeiro", "contabilidade"}},
	"contador":      {{"servicos", "contabilidade"}},
	"imobiliaria":   {{"servicos", "imobiliaria"}, {"imoveis", "imobiliaria"}},
	"imobiliária":   {{"servicos", "imobiliaria"}, {"imoveis", "imobiliaria"}},
	"despachante":   {{"servicos", "despachante"}},
	"seguradora":    {{"servicos", "seguradora"}, {"financeiro", "seguradora"}},
	"cartorio":      {{"servicos", "cartorio"}},
	"cartório":      {{"servicos", "cartorio"}},
	"consultoria":   {{"servicos", "consultoria"}},
	"coworking":     {{"servicos", "coworking"}},

	"escola":       {{"educacao", "escola"}, {"educacao", "colegio"}},
	"colegio":      {{"educacao", "colegio"}, {"educacao", "escola"}},
	"colégio":      {{"educacao", "colegio"}, {"educacao", "escola"}},
	"faculdade":    {{"educacao", "faculdade"}, {"educacao", "universidade"}},
	"universidade": {{"educacao", "universidade"}, {"educacao", "faculdade"}},
	"pre-escola":   {{"educacao", "pre-escola"}, {"educacao", "creche"}},
	"pre escola":   {{"educacao", "pre-escola"}, {"educacao", "creche"}},
	"idiomas":      {{"educacao", "idiomas"}, {"educacao", "curso-de-idiomas"}},
	"curso":        {{"educacao", "curso"}},
	"autoescola":   {{"educacao", "autoescola"}},

	"clinica":        {{"saude", "clinica"}, {"saude", "clinica-medica"}},
	"clínica":        {{"saude", "clinica"}, {"saude", "clinica-medica"}},
	"laboratorio":    {{"saude", "laboratorio"}, {"saude", "laboratorio-clinico"}},
	"laboratório":    {{"saude", "laboratorio"}, {"saude", "laboratorio-clinico"}},
	"fisioterapia":   {{"saude", "fisioterapia"}},
	"nutricao":       {{"saude", "nutricao"}, {"saude", "nutricionista"}},
	"nutrição":       {{"saude", "nutricao"}, {"saude", "nutricionista"}},
	"psicologia":     {{"saude", "psicologia"}, {"saude", "psicologo"}},
	"psicólogo":      {{"saude", "psicologia"}},
	"fonoaudiologia": {{"saude", "fonoaudiologia"}},
	"ortopedia":      {{"saude", "ortopedia"}, {"saude", "clinica"}},
	"dermatologia":   {{"saude", "dermatologia"}, {"saude", "clinica"}},
	"oftalmologia":   {{"saude", "oftalmologia"}, {"saude", "clinica"}},
	"cardiologia":    {{"saude", "cardiologia"}, {"saude", "clinica"}},

	"sorveteria":   {{"alimentacao", "sorveteria"}, {"alimentacao", "lanchonete"}},
	"doceria":      {{"alimentacao", "doceria"}, {"alimentacao", "confeitaria"}},
	"hortifruti":   {{"alimentacao", "hortifruti"}, {"comercio", "hortifruti"}},
	"mercearia":    {{"alimentacao", "mercearia"}, {"comercio", "mercearia"}},
	"acougue":      {{"alimentacao", "acougue"}, {"comercio", "acougue"}},
	"açougue":      {{"alimentacao", "acougue"}, {"comercio", "acougue"}},
	"peixaria":     {{"alimentacao", "peixaria"}, {"comercio", "peixaria"}},
	"cafeteria":    {{"alimentacao", "cafeteria"}, {"alimentacao", "lanchonete"}},
	"hamburgueria": {{"alimentacao", "hamburgueria"}, {"alimentacao", "lanchonete"}},
	"sushi":        {{"alimentacao", "restaurante-japones"}, {"alimentacao", "restaurante"}},
	"marmitaria":   {{"alimentacao", "marmitaria"}, {"alimentacao", "restaurante"}},

	"papelaria":               {{"comercio", "papelaria"}},
	"livraria":                {{"comercio", "livraria"}},
	"informatica":             {{"tecnologia", "informatica"}, {"comercio", "informatica"}},
	"informática":             {{"tecnologia", "informatica"}, {"comercio", "informatica"}},
	"celular":                 {{"tecnologia", "celular"}, {"comercio", "celular"}},
	"eletronicos":             {{"tecnologia", "eletronicos"}, {"comercio", "eletronicos"}},
	"eletrônicos":             {{"tecnologia", "eletronicos"}, {"comercio", "eletronicos"}},
	"farmacia de manipulacao": {{"saude", "farmacia-de-manipulacao"}, {"saude", "farmacia"}},
	"farmácia de manipulação": {{"saude", "farmacia-de-manipulacao"}, {"saude", "farmacia"}},
	"optica":                  {{"saude", "optica"}, {"comercio", "optica"}},
	"ótica":                   {{"saude", "optica"}, {"comercio", "optica"}},
	"joias":                   {{"moda-e-vestuario", "joalheria"}, {"comercio", "joalheria"}},
	"flores":                  {{"comercio", "floricultura"}, {"casa-e-jardim", "floricultura"}},
	"floricultura":            {{"comercio", "floricultura"}, {"casa-e-jardim", "floricultura"}},
	"instrumentos musicais":   {{"comercio", "instrumentos-musicais"}},
	"brinquedos":              {{"comercio", "brinquedos"}, {"comercio", "brinquedoteca"}},

	"pousada":           {{"turismo", "pousada"}, {"turismo", "hotel"}},
	"hostel":            {{"turismo", "hostel"}, {"turismo", "hotel"}},
	"buffet":            {{"eventos", "buffet"}, {"alimentacao", "buffet"}},
	"espaco de eventos": {{"eventos", "espaco-de-eventos"}},
	"espaço de eventos": {{"eventos", "espaco-de-eventos"}},
	"salao de festas":   {{"eventos", "salao-de-festas"}, {"eventos", "espaco-de-eventos"}},
	"fotografia":        {{"eventos", "fotografia"}, {"servicos", "fotografia"}},
}

// appLocalBuildAllURLs gera todas as URLs: subcategorias × páginas.
func appLocalBuildAllURLs(base, citySlug, stateSlug, querySlug string, maxPages int) []string {
	query := strings.ReplaceAll(querySlug, "-", " ")
	pairs, ok := appLocalCategoryMap[query]
	if !ok {
		pairs = [][2]string{
			{querySlug, querySlug},
			{querySlug + "s", querySlug},
		}
	}

	var urls []string
	for _, pair := range pairs {
		cat, sub := pair[0], pair[1]
		urls = append(urls, fmt.Sprintf("%s/%s-%s/%s/%s/", base, citySlug, stateSlug, cat, sub))
		for p := 2; p <= maxPages; p++ {
			urls = append(urls, fmt.Sprintf("%s/%s-%s/%s/%s/pagina/%d/", base, citySlug, stateSlug, cat, sub, p))
		}
	}
	return urls
}
