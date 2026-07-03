# Multi-stage build. SQLite here is modernc.org/sqlite (pure Go), so CGO is off and the runtime
# image needs no C libraries — just CA certificates for outbound HTTPS (Gemini/Groq/RSS). Migrations
# are embedded in the binary (go:embed), so nothing else needs to be copied into the final image.

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
# runtime stage needs no access to the Alpine package mirror. The app creates ./db on boot; the
# db/ volume (docker-compose.yaml) is mounted over it.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /app/api .

EXPOSE 3000

CMD ["./api"]
