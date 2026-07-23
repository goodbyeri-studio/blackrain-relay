# syntax=docker/dockerfile:1.7

# Backend-only image for the optional personal VPS mode. BuildKit caches keep
# repeated Go rebuilds fast without changing the standard local Dockerfile.
FROM golang:1.26.1-alpine AS builder

ENV GO111MODULE=on CGO_ENABLED=0 GOEXPERIMENT=greenteagc
ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}

WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN mkdir -p web/default/dist web/classic/dist && \
    echo '<!doctype html><html><head><title>dev</title></head><body>use frontend dev server</body></html>' > web/default/dist/index.html && \
    echo '<!doctype html><html><head><title>dev</title></head><body>use frontend dev server</body></html>' > web/classic/dist/index.html

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata wget \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=builder /build/new-api /
COPY LICENSE NOTICE THIRD-PARTY-LICENSES.md /licenses/
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/new-api"]
