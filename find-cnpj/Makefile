.PHONY: help build run test install-deps install-chromium clean

help: ## Mostra esta ajuda
	@echo "📋 Comandos disponíveis:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Compila o projeto
	@echo "🔨 Compilando..."
	go build -o go-lead main.go
	@echo "✅ Compilado com sucesso!"

run: ## Executa o programa (use: make run ARGS="nome empresa cnpj")
	@go run main.go $(ARGS)

test: ## Roda os testes
	@echo "🧪 Executando testes..."
	go test ./...

install-deps: ## Instala dependências do Go
	@echo "📦 Instalando dependências..."
	go mod download
	go mod tidy
	@echo "✅ Dependências instaladas!"

install-chromium: ## Instala Chromium (necessário para ChromeDP em servidores)
	@echo "⚠️  ATENÇÃO: ChromeDP requer Chromium instalado!"
	@echo ""
	@echo "🐧 Para Ubuntu/Debian:"
	@echo "  sudo apt-get update"
	@echo "  sudo apt-get install -y chromium-browser"
	@echo ""
	@echo "🎩 Para Fedora/RHEL:"
	@echo "  sudo dnf install -y chromium"
	@echo ""
	@echo "🍎 Para MacOS:"
	@echo "  brew install chromium"
	@echo ""
	@echo "🐋 Para Docker, adicione ao Dockerfile:"
	@echo "  RUN apt-get update && apt-get install -y chromium-browser"
	@echo ""
	@read -p "Deseja instalar agora? [s/N] " confirm; \
	if [ "$$confirm" = "s" ] || [ "$$confirm" = "S" ]; then \
		if command -v apt-get > /dev/null; then \
			sudo apt-get update && sudo apt-get install -y chromium-browser; \
		elif command -v dnf > /dev/null; then \
			sudo dnf install -y chromium; \
		else \
			echo "❌ Sistema não suportado. Instale manualmente."; \
		fi \
	fi

clean: ## Remove arquivos compilados
	@echo "🧹 Limpando..."
	rm -f go-lead
	@echo "✅ Limpeza concluída!"

server-setup: ## Guia de setup para servidor
	@echo "🖥️  SETUP EM SERVIDOR (PRODUÇÃO)"
	@echo "════════════════════════════════════════════════════"
	@echo ""
	@echo "1️⃣  Instalar Go (se não estiver instalado):"
	@echo "   wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz"
	@echo "   sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz"
	@echo "   export PATH=\$$PATH:/usr/local/go/bin"
	@echo ""
	@echo "2️⃣  Clonar o projeto:"
	@echo "   git clone <seu-repo>"
	@echo "   cd go-lead"
	@echo ""
	@echo "3️⃣  Instalar dependências:"
	@echo "   make install-deps"
	@echo ""
	@echo "4️⃣  ⚠️  IMPORTANTE: Instalar Chromium (SE usar ChromeDP):"
	@echo "   make install-chromium"
	@echo "   # OU manualmente:"
	@echo "   sudo apt-get install -y chromium-browser"
	@echo ""
	@echo "5️⃣  Compilar:"
	@echo "   make build"
	@echo ""
	@echo "6️⃣  Executar:"
	@echo "   ./go-lead \"nome empresa cnpj\""
	@echo ""
	@echo "════════════════════════════════════════════════════"
	@echo "💡 NOTA: DuckDuckGo e Sites de Consulta NÃO precisam"
	@echo "    de Chromium. Apenas se descomentar ChromeDP!"
	@echo "════════════════════════════════════════════════════"

# Atalhos úteis
dd: ## Testa com DuckDuckGo (exemplo: make dd QUERY="dimazzo arapongas cnpj")
	@go run main.go $(QUERY)

rate-limit-test: ## Testa rate limit do DuckDuckGo
	@go run test_rate_limit.go

process-list: ## Processa lista de empresas (use: make process-list FILE=empresas.txt)
	@if [ -z "$(FILE)" ]; then \
		echo "❌ Erro: especifique o arquivo com FILE=nome.txt"; \
		echo "Exemplo: make process-list FILE=empresas.txt"; \
		exit 1; \
	fi
	@go run process_list.go $(FILE)

process-list-safe: ## Processa lista com RETRY e salvamento contínuo (RECOMENDADO)
	@if [ -z "$(FILE)" ]; then \
		echo "❌ Erro: especifique o arquivo com FILE=nome.txt"; \
		echo "Exemplo: make process-list-safe FILE=empresas.txt"; \
		exit 1; \
	fi
	@go run process_list_safe.go $(FILE)

exemplo-lista: ## Processa arquivo de exemplo (versão rápida)
	@echo "⚠️  Usando versão rápida. Para versão SEGURA use: make exemplo-lista-safe"
	@go run process_list.go empresas_exemplo.txt

exemplo-lista-safe: ## Processa arquivo de exemplo (versão SEGURA - RECOMENDADO)
	@echo "✅ Usando versão SEGURA com retry e salvamento contínuo"
	@go run process_list_safe.go empresas_exemplo.txt
