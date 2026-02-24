# 🔍 Go Lead - Busca Inteligente de CNPJ

Sistema robusto de busca de CNPJ com múltiplas estratégias, fallback automático e enriquecimento de dados.

## 🎯 Funcionalidades

- ✅ Busca CNPJ a partir de queries textuais (ex: "dimazzo arapongas cnpj")
- ✅ Múltiplas estratégias com fallback automático
- ✅ Validação completa de CNPJ (dígitos verificadores)
- ✅ **Enriquecimento de dados: Sócios, Telefones, Razão Social e Nome Fantasia**
- ✅ **Fallback duplo: BrasilAPI → cnpj.biz**
- ✅ Extração de CNPJ de textos
- ✅ 6 estratégias diferentes implementadas
- ✅ 100% funcional sem custos (opções gratuitas)

## 🚀 Uso Rápido

```bash
# Busca simples (com dados adicionais)
go run main.go dimazzo arapongas cnpj

# Saída:
# ✅ CNPJ ENCONTRADO!
# 🔢 CNPJ: 04.309.163/0001-01
# 🏢 Razão Social: DI-MAZZO ARTIGOS DO VESTUARIO LTDA
# 🏪 Nome Fantasia: DI MAZZO
# 📞 Telefones: (43) 3252-1234
# 👥 Sócios (2):
#    1. NATHAN COSTA E SILVA
#    2. REGINA NUNES COSTA
```

## 📊 Estratégias Disponíveis

1. **DuckDuckGo Search** ⭐ - Gratuito, rápido, sem rate limit agressivo
2. **Sites de Consulta CNPJ** - Gratuito, backup confiável
3. **ChromeDP Scraping** - Gratuito, robusto (requer Chromium instalado - veja Makefile)

**REMOVIDO:** ~~Google Custom Search API~~ (era pago - $5/1000 queries)

📖 **Veja comparação detalhada em [ESTRATEGIAS.md](ESTRATEGIAS.md)**

---

## ⚡ Rate Limit e Performance

### Resultados de Teste Real (DuckDuckGo)
- ✅ **7 sucessos consecutivos** sem problemas
- ⏱️ **~700-900ms** por consulta
- 🚀 **~25 consultas/minuto** sustentável
- 💡 **Rate limit leve**: Delay de 1s entre consultas é seguro

### Para Listas de Empresas
```bash
# Processar lista de empresas
go run process_list.go empresas.txt

# Arquivo empresas.txt (um por linha):
# dimazzo arapongas
# magazine luiza
# coca cola brasil
```

**Configuração recomendada:**
- ✅ Delay de 1 segundo entre consultas
- ✅ Pausa de 5 segundos a cada 50 consultas
- ✅ Resultados salvos em CSV automaticamente

---

## 📊 Estratégias Disponíveis

1. **DuckDuckGo Search** - Gratuito, rápido, sem rate limit ⭐
2. **Sites de Consulta CNPJ** - Gratuito, backup confiável
3. ~~**Google Custom Search API**~~ - **REMOVIDO** (era pago)
4. **ChromeDP Scraping** - Gratuito, robusto, mais lento

---

## 🔎 Enriquecimento de Dados

Após encontrar um CNPJ, o sistema busca **automaticamente** dados adicionais:

### 📋 Dados Disponíveis:
- 🏢 **Razão Social**: Nome oficial da empresa
- 🏪 **Nome Fantasia**: Nome comercial
- 📞 **Telefones**: Todos os telefones cadastrados
- 👥 **Sócios**: Lista completa de sócios/administradores

### 🔄 Sistema de Fallback:
1. **BrasilAPI** (Primário) - API oficial, gratuita, rápida
2. **cnpj.biz** (Fallback) - Scraping quando BrasilAPI falhar

```bash
# Exemplo de saída completa:
✅ CNPJ ENCONTRADO!
🔢 CNPJ: 04.309.163/0001-01
🏢 Razão Social: DI-MAZZO ARTIGOS DO VESTUARIO LTDA
🏪 Nome Fantasia: DI MAZZO
👥 Sócios (2):
   1. NATHAN COSTA E SILVA
   2. REGINA NUNES COSTA
```

### 📄 Processamento em Lote
O CSV gerado inclui **todas as informações**:
```csv
Nome,CNPJ,CNPJ_Formatado,Razao_Social,Nome_Fantasia,Telefones,Socios,Fonte,Tempo_ms,Status
dimazzo arapongas,04309163000101,04.309.163/0001-01,DI-MAZZO ARTIGOS...,DI MAZZO,(43) 3252-1234,NATHAN COSTA E SILVA; REGINA NUNES COSTA,DuckDuckGo,807,sucesso
```

### 🧪 Testar Enriquecimento
```bash
# Teste completo de enriquecimento
go run test_enrichment.go

# Testa:
# - BrasilAPI
# - cnpj.biz (fallback)
# - Função automática EnrichCNPJData()
```

---

## 📦 Instalação

```bash
# Clone e entre no diretório
cd go-lead

# Instale as dependências
go mod download

# Execute
go run main.go
```

## 💡 Exemplos de Uso

### Linha de Comando
```bash
# Busca por nome
go run main.go dimazzo arapongas cnpj

# Qualquer empresa
go run main.go "coca cola brasil cnpj"
```

### Uso Programático

```go
package main

import (
    "context"
    "fmt"
    "go-lead/pkg/cnpj"
)

func main() {
    // Busca automática com fallback
    result := cnpj.SearchWithFallback(
        context.Background(),
        "dimazzo arapongas cnpj",
        cnpj.NewDuckDuckGoSearcher(),
        cnpj.NewCNPJSearcher(),
    )

    if result.Error == nil {
        fmt.Printf("CNPJ: %s\n", result.CNPJ.Formatted)
        fmt.Printf("Fonte: %s\n", result.Source)
    }
}
```

### Extrair de Texto

```go
// Extrair CNPJ de um texto
texto := "A empresa Dimazzo CNPJ: 04.309.163/0001-01 atua em Arapongas"
cnpj := cnpj.ExtractCNPJ(texto)

fmt.Println(cnpj.Formatted) // 04.309.163/0001-01
fmt.Println(cnpj.Number)    // 04309163000101
```

## ⚙️ Configuração Opcional

Para usar Google Custom Search API (opcional):

1. Copie o arquivo de exemplo:
```bash
cp .env.example .env
```

2. Configure suas credenciais em `.env`:
```bash
GOOGLE_API_KEY=sua_api_key
GOOGLE_CX=seu_custom_search_engine_id
```

3. Como obter:
   - **API Key**: https://console.cloud.google.com/apis/credentials
   - **CX**: https://programmablesearchengine.google.com/

## 🛠️ Dependências

- `github.com/chromedp/chromedp` - Automação de navegador
- `github.com/PuerkitoBio/goquery` - Parsing de HTML
- `github.com/joho/godotenv` - Variáveis de ambiente

## 📝 Validação de CNPJ

O sistema valida automaticamente CNPJs usando o algoritmo oficial:
- Verifica dígitos verificadores
- Rejeita CNPJs inválidos
- Formata corretamente (XX.XXX.XXX/XXXX-XX)

## 🎓 Estrutura do Projeto

```
go-lead/
├── main.go                          # Ponto de entrada
├── pkg/cnpj/
│   ├── cnpj.go                      # Validação e extração
│   ├── searcher.go                  # Interface e fallback
│   ├── google_search.go             # Google API
│   ├── brasilapi.go                 # BrasilAPI
│   ├── chromedp_search.go           # Web scraping
│   └── additional_searchers.go      # Outras estratégias
├── ESTRATEGIAS.md                   # Comparação detalhada
└── README.md                        # Este arquivo
```

## 🤝 Contribuindo

Sinta-se livre para:
- Adicionar novas estratégias de busca
- Melhorar a taxa de sucesso
- Reportar bugs
- Sugerir melhorias

## 📄 Licença

Projeto open source para uso livre.

---

**Desenvolvido com ❤️ em Go**
