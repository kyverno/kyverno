# Multi-stage docker build
# Build stage
FROM --platform=${BUILDPLATFORM} golang@sha256:5540a6a6b3b612c382accc545b3f6702de21e77b15d89ad947116c94b5f42993 AS base
WORKDIR /src
LABEL maintainer="Kyverno"

COPY go.* .

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

FROM --platform=${BUILDPLATFORM} tonistiigi/xx:1.1.1@sha256:23ca08d120366b31d1d7fad29283181f063b0b43879e1f93c045ca5b548868e9 AS xx

FROM base AS builder

# LD_FLAGS is passed as argument from Makefile. It will be empty, if no argument passed
ARG LD_FLAGS
ARG TARGETPLATFORM

COPY --from=xx / /

RUN --mount=type=bind,target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 xx-go build -o /output/kyverno -ldflags="${LD_FLAGS}" -v ./cmd/cli/kubectl-kyverno/

# Packaging stage
FROM ghcr.io/distroless/static:latest

LABEL maintainer="Kyverno"

COPY --from=builder /output/kyverno /

ENTRYPOINT ["/kyverno"]
