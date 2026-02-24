.PHONY: help build-all clean-all test-all install-all

help: ## Mostra esta mensagem de ajuda
	@echo "📋 Go Lead Monorepo - Comandos disponíveis:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "📁 Projetos individuais:"
	@echo "  cd find-cnpj && make help"
	@echo "  cd find-instagram && make help"

build-all: ## Compila todos os projetos
	@echo "🔨 Compilando todos os projetos..."
	@cd find-cnpj && make build
	@cd find-instagram && make build
	@echo "✅ Build concluído para todos os projetos!"

clean-all: ## Remove binários de todos os projetos
	@echo "🧹 Limpando todos os projetos..."
	@cd find-cnpj && make clean
	@cd find-instagram && make clean
	@echo "✅ Limpeza concluída!"

test-all: ## Executa testes de todos os projetos
	@echo "🧪 Executando testes de todos os projetos..."
	@cd find-cnpj && go test -v ./... || true
	@cd find-instagram && go test -v ./... || true
	@echo "✅ Testes concluídos!"

install-all: ## Instala dependências de todos os projetos
	@echo "📦 Instalando dependências..."
	@cd find-cnpj && go mod download && go mod tidy
	@cd find-instagram && go mod download && go mod tidy
	@echo "✅ Dependências instaladas!"

fmt-all: ## Formata código de todos os projetos
	@echo "🎨 Formatando código..."
	@cd find-cnpj && go fmt ./...
	@cd find-instagram && go fmt ./...
	@echo "✅ Código formatado!"

vet-all: ## Analisa código de todos os projetos
	@echo "🔍 Analisando código..."
	@cd find-cnpj && go vet ./... || true
	@cd find-instagram && go vet ./... || true
	@echo "✅ Análise concluída!"

status: ## Mostra status dos projetos
	@echo "📊 Status dos Projetos:"
	@echo ""
	@echo "🔍 find-cnpj:"
	@if [ -f find-cnpj/go-lead ]; then echo "  ✅ Compilado"; else echo "  ❌ Não compilado"; fi
	@echo ""
	@echo "📱 find-instagram:"
	@if [ -f find-instagram/find-instagram ]; then echo "  ✅ Compilado"; else echo "  ❌ Não compilado"; fi
