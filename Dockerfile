# ==============================================================================
# PROJECT MITTENS: Hardened Multi-Stage Distroless Dockerfile
# ==============================================================================
# Stage 1: Build React / TypeScript Mission Control Web UI
# ==============================================================================
FROM node:20-alpine AS web-builder

WORKDIR /web

# Cache dependency installations
COPY web/package.json web/package-lock.json* ./
RUN npm ci || npm install

# Copy web source and compile production bundle
COPY web/ ./
RUN npm run build

# ==============================================================================
# Stage 2: Build & Compile Statically Linked Go Binaries
# ==============================================================================
FROM golang:alpine AS go-builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Leverage Docker cache layer for dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy full source tree and embed compiled frontend dist
COPY . .
COPY --from=web-builder /web/dist ./web/dist

# Compile static binaries for server, batch optimizer, and tournament harness
ENV CGO_ENABLED=0 \
    GOOS=linux

RUN go build -trimpath -ldflags="-s -w -extldflags '-static'" -o /bin/mittens-server ./cmd/server && \
    go build -trimpath -ldflags="-s -w -extldflags '-static'" -o /bin/mittens-opt ./cmd/optimizer && \
    go build -trimpath -ldflags="-s -w -extldflags '-static'" -o /bin/mittens-tournament ./cmd/tournament

# ==============================================================================
# Stage 3: Minimal, Hardened, Non-Root Distroless Runtime Image
# ==============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

LABEL maintainer="Project Mittens Engineering Team <eng@optimaldynamics.com>" \
      description="Project Mittens MOMDP Carrier Optimization Engine & Mission Control UI" \
      version="1.0.0"

# Copy certificates, timezone database, and compiled binaries
COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=go-builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=go-builder /bin/mittens-server /usr/local/bin/mittens-server
COPY --from=go-builder /bin/mittens-opt /usr/local/bin/mittens-opt
COPY --from=go-builder /bin/mittens-tournament /usr/local/bin/mittens-tournament
COPY --from=go-builder /src/api/openapi.yaml /etc/project-mittens/openapi.yaml

# Copy static frontend assets
COPY --from=web-builder /web/dist /var/www/mittens/dist

# Configure static assets location for embedded HTTP server
ENV MITTENS_STATIC_DIR=/var/www/mittens/dist


# Non-root user (distroless nonroot UID=65532, GID=65532)
USER nonroot:nonroot

# Expose HTTP API & Prometheus metrics port
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/mittens-server"]
CMD ["-host", "0.0.0.0", "-port", "8080"]
