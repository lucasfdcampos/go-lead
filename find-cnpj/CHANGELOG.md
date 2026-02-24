# 📝 Changelog

## [2.0.0] - 2026-02-24

### ✅ Adicionado
- 📊 **Teste de Rate Limit** (`test_rate_limit.go`)
  - Testa DuckDuckGo com 20 consultas sequenciais
  - Análise completa de performance
  - Resultados: ~25 consultas/minuto sustentável

- 📋 **Processamento em Lote** (`process_list.go`)
  - Processa listas de empresas de arquivo .txt
  - Delay inteligente (1s entre consultas, 5s a cada 50)
  - Exporta resultados para CSV automaticamente
  - Progress bar com status em tempo real

- 🚀 **Makefile Completo**
  - `make help` - Lista todos comandos
  - `make build` - Compila o projeto
  - `make rate-limit-test` - Testa rate limit
  - `make process-list FILE=arquivo.txt` - Processa lista
  - `make exemplo-lista` - Processa exemplo pronto
  - `make server-setup` - Guia de deploy em servidor
  - `make install-chromium` - Ajuda a instalar Chromium

- 📚 **Documentação**
  - `DEPLOY.md` - Guia completo de deploy
  - `RATE_LIMIT_ANALYSIS.md` - Análise detalhada do teste
  - `empresas_exemplo.txt` - Lista de exemplo

### ❌ Removido
- 🚫 **Google Custom Search API** (era pago - $5/1000 queries)
  - Removido `pkg/cnpj/google_search.go`
  - Removido `.env.example`
  - Removido menções no código e documentação
  
### 🔧 Modificado
- ✏️ README.md - Atualizado com info de rate limit
- ✏️ main.go - Removido setup do Google API
- ✏️ ESTRATEGIAS.md - Atualizado comparações

### 📊 Resultados do Teste
- ✅ 7 CNPJs encontrados consecutivamente
- ⏱️ Tempo médio: 686ms por consulta
- 🚀 Throughput: ~25 consultas/minuto
- 💡 Rate limit: LEVE (delay de 1s é seguro)

### 🎯 Configuração Atual
**Stack 100% Gratuita:**
1. DuckDuckGo Search (principal)
2. Sites de Consulta CNPJ (backup)
3. ChromeDP (opcional - requer Chromium)

**Dependências de Servidor:**
- ✅ Go 1.24+
- ⚠️ Chromium (APENAS se usar ChromeDP)
- ❌ Nenhuma API key necessária!

---

## [1.0.0] - 2026-02-24

### Lançamento Inicial
- 6 estratégias de busca implementadas
- Sistema de fallback automático
- Validação completa de CNPJ
- Extração de CNPJ de textos
- Suporte a múltiplas fontes
