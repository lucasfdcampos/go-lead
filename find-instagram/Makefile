.PHONY: help build run clean test install

help: ## Mostra esta mensagem de ajuda
	@echo "📋 Comandos disponíveis:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Compila o projeto
	@echo "🔨 Compilando..."
	@go build -o find-instagram main.go
	@echo "✅ Build concluído!"

run: build ## Compila e executa com exemplo
	@echo "🚀 Executando exemplo..."
	@./find-instagram "Magazine Luiza"

clean: ## Remove binários e arquivos temporários
	@echo "🧹 Limpando..."
	@rm -f find-instagram
	@rm -f *.csv
	@echo "✅ Limpeza concluída!"

test: ## Executa os testes
	@echo "🧪 Executando testes..."
	@go test -v ./...

install: ## Instala as dependências
	@echo "📦 Instalando dependências..."
	@go mod download
	@go mod tidy
	@echo "✅ Dependências instaladas!"

fmt: ## Formata o código
	@echo "🎨 Formatando código..."
	@go fmt ./...
	@echo "✅ Código formatado!"

vet: ## Analisa o código
	@echo "🔍 Analisando código..."
	@go vet ./...
	@echo "✅ Análise concluída!"
