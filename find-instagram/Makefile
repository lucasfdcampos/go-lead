.PHONY: help build run clean test install exemplo process-list

help: ## Mostra esta mensagem de ajuda
	@echo "📋 Comandos disponíveis:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Compila o projeto
	@echo "🔨 Compilando find-instagram..."
	@go build -o find-instagram main.go
	@echo "✅ Build concluído!"

build-list: ## Compila o processador de listas
	@echo "🔨 Compilando process-list..."
	@go build -o process-list process_list.go
	@echo "✅ Build concluído!"

build-all: build build-list ## Compila todos os binários
	@echo "✅ Todos os binários compilados!"

run: build ## Compila e executa com exemplo
	@echo "🚀 Executando exemplo..."
	@./find-instagram "Magazine Luiza"

exemplo: build ## Executa exemplo com Magazine Luiza
	@./find-instagram "Magazine Luiza"

exemplo-dimazzo: build ## Executa exemplo com Dimazzo Arapongas
	@./find-instagram "dimazzo arapongas"

process-list: build-list ## Processa lista de empresas (use LISTA=arquivo.txt)
	@if [ -z "$(LISTA)" ]; then \
		echo "❌ Uso: make process-list LISTA=arquivo.txt"; \
		exit 1; \
	fi
	@./process-list $(LISTA)

clean: ## Remove binários e arquivos temporários
	@echo "🧹 Limpando..."
	@rm -f find-instagram process-list
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
