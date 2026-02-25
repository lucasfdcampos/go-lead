.PHONY: run build tidy docker-up docker-down lint test

# ─── Local development ────────────────────────────────────────────────────────

run:
	@[ -f .env ] && export $$(grep -v '^#' .env | xargs); go run ./main.go

build:
	go build -o bin/lead-api ./main.go

tidy:
	go mod tidy

# ─── Docker ───────────────────────────────────────────────────────────────────

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f api

# ─── Quality ──────────────────────────────────────────────────────────────────

lint:
	golangci-lint run ./...

test:
	go test ./... -v -timeout 60s
