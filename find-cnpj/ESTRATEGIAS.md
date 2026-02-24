# Busca de CNPJ - Estratégias e Comparações

Este projeto implementa múltiplas estratégias para buscar CNPJ de estabelecimentos a partir de queries textuais como "dimazzo arapongas cnpj".

## 🎯 Estratégias Implementadas

### 1. **DuckDuckGo Search** 🦆
Busca usando DuckDuckGo HTML (sem JavaScript).

**Prós:**
- ✅ 100% Gratuito
- ✅ Sem necessidade de API key
- ✅ Sem rate limit agressivo
- ✅ Rápido (3-5 segundos)
- ✅ Não requer JavaScript/navegador
- ✅ Boa taxa de sucesso

**Contras:**
- ❌ Resultados podem variar em qualidade
- ❌ Pode não encontrar CNPJs menos conhecidos
- ❌ Dependente de parsing de HTML (pode quebrar se mudarem o layout)

**Quando usar:** Primeira opção recomendada para a maioria dos casos.

---

### 2. **Sites de Consulta CNPJ** 🌐
Faz scraping de sites especializados em consulta de CNPJ.

**Prós:**
- ✅ Gratuito
- ✅ Dados geralmente confiáveis
- ✅ Funciona sem APIs

**Contras:**
- ❌ Pode ter captcha
- ❌ Pode ter rate limit
- ❌ Sites podem sair do ar
- ❌ Depende de parsing específico

**Quando usar:** Como segunda opção ou para validação cruzada.

---

### 3. **Google Custom Search API** 🔍
Usa a API oficial do Google para buscar.

**Prós:**
- ✅ Resultados de alta qualidade
- ✅ API oficial e estável
- ✅ Melhor cobertura de sites
- ✅ Configurável (filtros, região, etc)
- ✅ JSON estruturado

**Contras:**
- ❌ Requer API key (Google Cloud)
- ❌ 100 queries grátis/dia, depois **$5 por 1000 queries**
- ❌ Requer configuração de Custom Search Engine (CX)
- ❌ Setup mais complexo

**Quando usar:** Quando precisa de resultados de máxima qualidade e tem orçamento.

**Como configurar:**
1. Criar projeto no Google Cloud Console
2. Ativar Custom Search API
3. Criar Custom Search Engine em https://programmablesearchengine.google.com/
4. Configurar `.env`:
```bash
GOOGLE_API_KEY=sua_api_key_aqui
GOOGLE_CX=seu_cx_aqui
```

---

### 4. **Web Scraping com ChromeDP** 🤖
Usa navegador headless (Chrome) para fazer scraping do Google.

**Prós:**
- ✅ Gratuito
- ✅ Sem necessidade de API
- ✅ Executa JavaScript (sites dinâmicos)
- ✅ Simula navegador real (evita alguns bloqueios)
- ✅ Melhor taxa de sucesso que HTTP simples
- ✅ Pode interagir com a página (clicar, rolar, etc)

**Contras:**
- ❌ Requer Chrome/Chromium instalado
- ❌ Mais lento (10-20 segundos)
- ❌ Mais consumo de recursos (RAM, CPU)
- ❌ Pode ser bloqueado por anti-bot
- ❌ Google pode detectar e limitar

**Quando usar:** Quando outras opções falharem ou para sites que requerem JavaScript.

**Requisitos:**
```bash
# Ubuntu/Debian
sudo apt-get install chromium-browser

# Fedora
sudo dnf install chromium

# MacOS
brew install chromium
```

---

### 5. **BrasilAPI** 🇧🇷
API pública brasileira para consultar dados de CNPJ.

**Prós:**
- ✅ 100% Gratuita
- ✅ API oficial brasileira
- ✅ Dados atualizados da Receita Federal
- ✅ JSON estruturado com muitos dados
- ✅ Sem necessidade de API key

**Contras:**
- ❌ **Requer CNPJ exato** (não busca por nome)
- ❌ Útil apenas para validação
- ❌ Rate limit pode ser aplicado

**Quando usar:** Para validar um CNPJ que você já extraiu de outra fonte.

---

### 6. **ReceitaWS** 📊
API de consulta de CNPJ (terceirizada).

**Prós:**
- ✅ Gratuita (com limite)
- ✅ Dados da Receita Federal
- ✅ JSON estruturado

**Contras:**
- ❌ Rate limit agressivo (3 requisições/minuto)
- ❌ Requer CNPJ exato
- ❌ Pode ficar indisponível

**Quando usar:** Backup para BrasilAPI.

---

## 📊 Comparação Rápida

| Estratégia | Custo | Velocidade | Taxa Sucesso | Setup | Recomendação |
|-----------|-------|------------|--------------|-------|--------------|
| DuckDuckGo | Grátis | ⚡⚡⚡ | 80% | Fácil | ⭐⭐⭐⭐⭐ |
| Sites CNPJ | Grátis | ⚡⚡ | 70% | Fácil | ⭐⭐⭐⭐ |
| Google API | Pago | ⚡⚡⚡ | 95% | Médio | ⭐⭐⭐⭐ |
| ChromeDP | Grátis | ⚡ | 90% | Médio | ⭐⭐⭐ |
| BrasilAPI | Grátis | ⚡⚡⚡ | N/A* | Fácil | ⭐⭐⭐ |
| ReceitaWS | Grátis | ⚡⚡ | N/A* | Fácil | ⭐⭐ |

\* Requer CNPJ exato, não faz busca por nome

---

## 🎯 Estratégia Recomendada (Fallback)

A ordem ideal para tentar as estratégias é:

```
1. DuckDuckGo Search (rápido, gratuito, boa taxa de sucesso)
   ↓ (se falhar)
2. Sites de Consulta CNPJ (gratuito, backup rápido)
   ↓ (se falhar)
3. Google Custom Search API (se configurado e tem orçamento)
   ↓ (se falhar)
4. ChromeDP Scraping (mais lento mas robusto)
```

---

## 🚀 Como Usar

### Uso Básico
```bash
go run main.go dimazzo arapongas cnpj
```

### Uso Programático
```go
import "go-lead/pkg/cnpj"

// Busca automática com fallback
result := cnpj.SearchWithFallback(
    context.Background(),
    "dimazzo arapongas cnpj",
    cnpj.NewDuckDuckGoSearcher(),
    cnpj.NewCNPJSearcher(),
    cnpj.NewChromeDPSearcher(true),
)

if result.Error == nil {
    fmt.Printf("CNPJ: %s\n", result.CNPJ.Formatted)
}
```

### Extrair CNPJ de Texto
```go
texto := "A empresa tem CNPJ 04.309.163/0001-01"
cnpj := cnpj.ExtractCNPJ(texto)
fmt.Println(cnpj.Formatted) // 04.309.163/0001-01
fmt.Println(cnpj.Number)    // 04309163000101
```

---

## 🛠️ Configuração

### Arquivo `.env` (opcional)
```bash
# Google Custom Search (opcional)
GOOGLE_API_KEY=your_api_key_here
GOOGLE_CX=your_custom_search_engine_id

# Outras configurações futuras...
```

### Dependências
```bash
go get github.com/chromedp/chromedp
go get github.com/PuerkitoBio/goquery
go get github.com/joho/godotenv
```

---

## 📝 Validação de CNPJ

O sistema implementa validação completa de CNPJ usando o algoritmo de dígitos verificadores. Apenas CNPJs válidos são retornados.

```go
// Validação automática
cnpj := cnpj.ExtractCNPJ("04.309.163/0001-01")
// Se retornar nil, o CNPJ é inválido

// Validação manual
isValid := cnpj.IsValidCNPJ("04309163000101")
```

---

## 🎓 Conclusão

**Para a maioria dos casos:**
- Use **DuckDuckGo** como primeira opção (grátis, rápido, sem setup)
- Mantenha **ChromeDP** como fallback final (robusto mas lento)
- Configure **Google API** se precisar de máxima qualidade e tem orçamento

**O sistema atual está configurado para máxima eficiência sem custos!** 🎉
