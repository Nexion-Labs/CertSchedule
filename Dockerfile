# --- Stage 1: build the React frontend ---
FROM node:22-slim AS web-build
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- Stage 2: build the Go binary with the frontend embedded ---
FROM golang:1.26-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-build /web/dist/ ./internal/webui/dist/
RUN CGO_ENABLED=0 go build -o /out/certschedule ./cmd/server

# --- Stage 3: runtime image with certbot + DNS plugins installed ---
FROM python:3.12-slim-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends \
      certbot \
      python3-certbot-dns-cloudflare \
      python3-certbot-dns-route53 \
      ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --create-home --uid 10001 certschedule
WORKDIR /app
COPY --from=go-build /out/certschedule /app/certschedule

RUN mkdir -p /app/data && chown -R certschedule:certschedule /app/data
USER certschedule

ENV HTTP_ADDR=:8080 \
    SQLITE_PATH=/app/data/certschedule.db \
    CERTBOT_DATA_DIR=/app/data/certbot \
    CERTBOT_WEBROOT=/app/data/webroot

EXPOSE 8080
ENTRYPOINT ["/app/certschedule"]
