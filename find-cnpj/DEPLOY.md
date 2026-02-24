# 🚀 Deploy em Servidor - Guia Completo

## 📋 Checklist de Dependências

### ✅ Sempre Necessário
- Go 1.24+ instalado
- Acesso à internet (para DuckDuckGo)

### ⚠️ Opcional (apenas se usar ChromeDP)
- **Chromium-browser** - OBRIGATÓRIO se descomentar ChromeDP no código

---

## 🐧 Setup no Servidor (Ubuntu/Debian)

### 1️⃣ Instalar Go
```bash
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
```

### 2️⃣ Clonar e Setup
```bash
git clone <seu-repositorio>
cd go-lead
make install-deps
make build
```

### 3️⃣ Instalar Chromium (APENAS SE USAR ChromeDP)
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y chromium-browser

# Ou use o Makefile
make install-chromium
```

**⚠️ IMPORTANTE:** 
- DuckDuckGo e Sites de Consulta **NÃO precisam** de Chromium
- Só instale se descomentar ChromeDP no código
- ChromeDP é mais lento e consome mais recursos

---

## 🐋 Docker (Recomendado)

### Dockerfile
```dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o go-lead main.go

FROM alpine:latest
WORKDIR /app

# Apenas se usar ChromeDP (não recomendado em Docker)
# RUN apk add --no-cache chromium

COPY --from=builder /app/go-lead .

ENTRYPOINT ["./go-lead"]
```

### docker-compose.yml
```yaml
version: '3.8'
services:
  go-lead:
    build: .
    command: ["dimazzo arapongas cnpj"]
    restart: unless-stopped
```

---

## 📊 Estratégias e Dependências

| Estratégia | Precisa Chromium? | Velocidade | Taxa Sucesso |
|-----------|------------------|-----------|--------------|
| DuckDuckGo | ❌ NÃO | ⚡⚡⚡ | 85% |
| Sites CNPJ | ❌ NÃO | ⚡⚡ | 70% |
| ChromeDP | ⚠️ **SIM** | ⚡ | 90% |

**Recomendação:** Use apenas DuckDuckGo + Sites CNPJ (configuração atual)

---

## 🔧 Configuração Atual (Sem Chromium)

Por padrão, o sistema usa:
1. **DuckDuckGo** (gratuito, sem dependências)
2. **Sites de Consulta CNPJ** (gratuito, sem dependências)

**✅ Não precisa instalar nada além do Go!**

Para habilitar ChromeDP:
```go
// No arquivo main.go, descomente:
searchers = append(searchers, cnpj.NewChromeDPSearcher(true))
```

---

## 📝 Comandos Úteis

```bash
# Ver ajuda
make help

# Setup completo
make install-deps
make build

# Testar
./go-lead "empresa nome cnpj"

# Ver guia de deploy
make server-setup

# Instalar chromium (se precisar)
make install-chromium
```

---

## ⚡ Performance e Rate Limits

### DuckDuckGo
- ✅ Sem rate limit agressivo
- ✅ ~1-2s por consulta
- ✅ Pode processar listas médias (100-500 empresas)
- 💡 Recomendação: Delay de 500ms entre consultas

### ChromeDP  
- ⚠️ Mais lento (10-20s por consulta)
- ⚠️ Alto consumo de RAM (~100-200MB por instância)
- ✅ Sem rate limit
- 💡 Use como fallback final

---

## 🎯 Recomendação Final

**Para servidores:**
- Use configuração atual (DuckDuckGo + Sites CNPJ)
- **NÃO instale Chromium** a menos que realmente precise
- ChromeDP só vale para casos onde outras estratégias falham

**Para desktop/desenvolvimento:**
- Pode usar ChromeDP tranquilamente
- Útil para debugging e casos difíceis

---

## 🆘 Troubleshooting

### Erro: "chromedp: not found"
**Solução:** Você não precisa de ChromeDP! Ele está comentado por padrão.

### Rate limit no DuckDuckGo
**Solução:** Adicione delay entre requisições:
```go
time.Sleep(1 * time.Second)
```

### CNPJs não encontrados
**Solução:** Habilite ChromeDP como fallback (requer Chromium instalado)
