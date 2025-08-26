# Step 1: Build
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /src

RUN apk add --no-cache git ca-certificates

# Use Docker BuildKit cache for modules
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Build all three tools for the target arch
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/any-sync-network ./any-sync-network && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/any-sync-netcheck ./any-sync-netcheck && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/anyconf ./anyconf


# Step 2: Runtime
FROM alpine:3.22.1

# OCI labels
LABEL org.opencontainers.image.title="any-sync-tools" \
      org.opencontainers.image.description="Configuration tools for Any-Sync network" \
      org.opencontainers.image.source="https://github.com/anyproto/any-sync-tools" \
      org.opencontainers.image.licenses="MIT"

WORKDIR /app

COPY --from=builder /out/any-sync-network /usr/local/bin/any-sync-network
COPY --from=builder /out/any-sync-netcheck /usr/local/bin/any-sync-netcheck
COPY --from=builder /out/anyconf /usr/local/bin/anyconf
COPY --from=builder /src/any-sync-network/defaultTemplate.yml .

CMD ["any-sync-network", "--help"]
