# 🚀 Go Lead - Monorepo

Monorepo contendo ferramentas automatizadas para busca de informações de empresas e estabelecimentos comerciais.

## 📂 Projetos

### 🔍 [find-cnpj](./find-cnpj)
Sistema automatizado para encontrar CNPJs (Cadastro Nacional de Pessoa Jurídica) de estabelecimentos a partir de buscas simples.

**Status:** ✅ Completo e funcional

**Funcionalidades:**
- Busca de CNPJ por nome da empresa
- 6 estratégias de fallback automático
- Processamento em lote com CSV
- Rate limit handling
- Sistema de retry
- 100% free (sem API keys pagas)

**Exemplo:**
```bash
cd find-cnpj
./go-lead "dimazzo arapongas"
# Output: 04.309.163/0001-01
```

### 📱 [find-instagram](./find-instagram)
Sistema automatizado para encontrar perfis do Instagram de estabelecimentos comerciais.

**Status:** 🚧 Em desenvolvimento

**Objetivo:**
Dado o nome de uma empresa, retornar o handle do Instagram (ex: "@magazineluiza").

## 🛠️ Tecnologias

- **Go 1.24+**
- Web scraping
- APIs públicas
- Pattern matching

## 🚀 Quick Start

### find-cnpj
```bash
cd find-cnpj
make help          # Ver comandos disponíveis
make exemplo       # Executar exemplo
make processo-lista LISTA=empresas.txt  # Processar lista
```

### find-instagram
```bash
cd find-instagram
make help          # Ver comandos disponíveis
make run           # Executar exemplo
```

## 📦 Instalação

```bash
# Clone o repositório
git clone https://github.com/lucasfdcampos/go-lead.git
cd go-lead

# find-cnpj
cd find-cnpj
go mod download
make build

# find-instagram
cd ../find-instagram
go mod download
make build
```

## 🏗️ Estrutura

```
go-lead/
├── find-cnpj/          # Busca de CNPJ
│   ├── pkg/            # Pacotes internos
│   ├── main.go         # Entry point
│   └── docs/           # Documentação
│
└── find-instagram/     # Busca de Instagram
    ├── main.go         # Entry point
    └── README.md       # Documentação
```

## 🤝 Contribuição

Cada projeto tem sua própria documentação. Consulte os READMEs específicos:
- [find-cnpj/README.md](./find-cnpj/README.md)
- [find-instagram/README.md](./find-instagram/README.md)

## 📄 Licença

MIT License

## 📚 Documentação

- **find-cnpj**: Documentação completa com 8 arquivos .md
- **find-instagram**: Em desenvolvimento

## 🎯 Roadmap

- [x] Sistema de busca de CNPJ
- [x] Processamento em lote
- [x] Rate limit handling
- [ ] Sistema de busca de Instagram
- [ ] Sistema de busca de WhatsApp
- [ ] API REST unificada
- [ ] Dashboard web
