# Multi-stage build. The Postgres driver is pgx (pure Go), so CGO stays off and the runtime image
# needs no C libraries — just CA certificates for outbound HTTPS (Gemini/Groq/RSS). Migrations are
# embedded in the binary (go:embed), so nothing else needs to be copied into the final image.

FROM golang:1.26-alpine AS builder

WORKDIR /app

# Cache dependencies separately from the source for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static binary: no CGO, stripped and trimmed to keep the image small.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o api .

FROM alpine:3.21

WORKDIR /app

# CA certificates for outbound HTTPS (Gemini/Groq/RSS). Copied from the builder (the golang image
# already ships them — that is how `go mod download` fetched over HTTPS) instead of `apk add`, so the
# runtime stage needs no access to the Alpine package mirror. The container is stateless: all state
# lives in Postgres, reached via DATABASE_URL.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /app/api .

EXPOSE 3000

CMD ["./api"]
