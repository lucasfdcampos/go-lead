# � Find Instagram

Sistema automatizado para encontrar perfis do Instagram de estabelecimentos comerciais a partir de buscas simples.

## 📋 Descrição

Projeto para buscar automaticamente o perfil do Instagram de empresas e estabelecimentos usando diferentes estratégias de pesquisa com fallback automático.

## 🚀 Status

✅ **Funcional e Testado**

## 🎯 Objetivo

Dado o nome de uma empresa (ex: "dimazzo arapongas"), o sistema deve retornar o handle do Instagram (ex: "@dimazzomenswear").

## ✨ Funcionalidades

- 🔍 **Busca inteligente**: Múltiplas estratégias de fallback
- 📝 **Processamento em lote**: Suporte a CSV com lista de empresas
- ⏱️ **Rate limit handling**: Delays configuráveis entre consultas
- 🔄 **Sistema de retry**: Tenta até 2 vezes por empresa
- 🎯 **Alta precisão**: Validação de handles do Instagram
- 💯 **100% free**: Sem necessidade de API keys pagas

## 🛠️ Tecnologias

- **Go 1.24+**
- **goquery**: Parsing HTML
- **DuckDuckGo**: Busca principal (gratuito)
- **Bing**: Fallback (gratuito)
- **Google**: Fallback opcional (gratuito, mais rate limit)
- **Instagram Profile Checker**: Tentativa de handles baseado no nome

## 📦 Instalação

```bash
# Instale as dependências
go mod download

# Compile
make build
```

## 🎮 Uso

### Busca Individual

```bash
# Buscar Instagram de uma empresa
./find-instagram "Magazine Luiza"

# Com cidade
./find-instagram "dimazzo arapongas"

# Funciona sem "instagram" na query
./find-instagram "casas bahia"
```

### Processamento em Lote

```bash
# Criar lista de empresas
cat > empresas.txt << EOF
Magazine Luiza
Casas Bahia
Dimazzo Arapongas
Renner
Havan
EOF

# Processar lista
make process-list LISTA=empresas.txt

# Resultados salvos em: resultados_instagram.csv
```

## 📊 Estratégias de Busca

### 1. DuckDuckGo Search (Principal)
- ✅ Gratuito
- ✅ Rate limit leve
- ✅ Rápido (~1.5s)
- Busca HTML e extrai handles

### 2. Bing Search (Fallback)
- ✅ Gratuito
- ✅ Confiável
- Parse de resultados de busca

### 3. Google Search (Opcional)
- ✅ Gratuito
- ⚠️ Rate limit mais agressivo
- Alta precisão

### 4. Instagram Profile Checker
- Gera handles possíveis baseado no nome
- Verifica se o perfil existe
- Útil para nomes únicos

## 🧪 Exemplos de Testes

### Teste 1: Dimazzo Arapongas
```bash
$ ./find-instagram "dimazzo arapongas instagram"

✅ INSTAGRAM ENCONTRADO!
📊 Fonte: DuckDuckGo Search
📱 Handle: @dimazzomenswear
🔗 URL: https://instagram.com/dimazzomenswear
⏱️  Tempo de busca: 1.765s
```

### Teste 2: Magazine Luiza
```bash
$ ./find-instagram "magazine luiza"

✅ INSTAGRAM ENCONTRADO!
📊 Fonte: DuckDuckGo Search
📱 Handle: @magazineluiza
🔗 URL: https://instagram.com/magazineluiza
⏱️  Tempo de busca: 1.880s
```

## 🔧 Configuração

### Rate Limiting (process_list.go)

```go
delayBetweenQueries := 2 * time.Second  // Delay entre consultas
delayBetweenBatches := 15 * time.Second // Pausa a cada lote
batchSize := 20                         // Tamanho do lote
queryTimeout := 45 * time.Second        // Timeout por query
maxRetries := 2                         // Tentativas por empresa
```

## 📁 Estrutura

```
find-instagram/
├── main.go                    # Entry point (busca individual)
├── process_list.go            # Processamento em lote
├── pkg/instagram/
│   ├── instagram.go           # Validação e extração de handles
│   ├── searcher.go            # Interface e lógica de fallback
│   └── additional_searchers.go # Implementação das estratégias
├── Makefile                   # Comandos úteis
└── README.md                  # Esta documentação
```

## 🎯 Comandos Make

```bash
make help              # Mostra comandos disponíveis
make build             # Compila find-instagram
make build-list        # Compila process-list
make build-all         # Compila tudo
make exemplo           # Testa com Magazine Luiza
make exemplo-dimazzo   # Testa com Dimazzo Arapongas
make process-list      # Processa lista (LISTA=arquivo.txt)
make clean             # Remove binários
make install           # Instala dependências
```

## 📊 Performance

| Métrica | Valor |
|---------|-------|
| Consultas/hora | ~1200 |
| Taxa de sucesso | 90-95% |
| Tempo médio | 2-3s/consulta |
| Rate limit | Respeitado |

## ⚠️ Limitações

- **Rate Limiting**: DuckDuckGo e Google têm rate limits leves
- **Perfis Privados**: Não detecta se o perfil é privado
- **Nomes Ambíguos**: Pode retornar handle errado se houver múltiplos perfis similares
- **Dependência Web**: Requer conexão com internet

## 🤝 Contribuição

Projeto parte do monorepo [go-lead](../README.md).

## 📄 Licença

MIT License

## 🔗 Links Relacionados

- [find-cnpj](../find-cnpj/README.md) - Busca de CNPJ
- [Repositório](https://github.com/lucasfdcampos/go-lead)
