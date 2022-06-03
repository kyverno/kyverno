# Multi-stage docker build
# Build stage
FROM --platform=${BUILDPLATFORM} golang@sha256:724abf4dd44985d060f7aa91af5211eb2052491424bd497ba3ddc31f7cee969d AS base
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
    CGO_ENABLED=0 xx-go build -o /output/kyvernopre -ldflags="${LD_FLAGS}" -v ./cmd/initContainer/

# Packaging stage
FROM scratch

LABEL maintainer="Kyverno"

COPY --from=builder /output/kyvernopre /
COPY --from=builder /etc/passwd /etc/passwd

USER 10001

ENTRYPOINT ["./kyvernopre"]
