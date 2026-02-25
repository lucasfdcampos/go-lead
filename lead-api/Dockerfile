# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /workspace

# Copy only what lead-api needs (uses replace directives → ../find-*)
COPY find-cnpj/    find-cnpj/
COPY find-instagram/ find-instagram/
COPY find-leads/   find-leads/
COPY lead-api/     lead-api/

WORKDIR /workspace/lead-api
# GOWORK=off ensures we rely on the replace directives in go.mod, not go.work
RUN GOWORK=off go build -o /lead-api ./main.go

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /lead-api /lead-api

EXPOSE 8080
ENTRYPOINT ["/lead-api"]
