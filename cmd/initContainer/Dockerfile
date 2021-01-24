
# Multi-stage docker build
# Build stage
FROM golang:1.14 AS builder

LABEL maintainer="Kyverno"

# LD_FLAGS is passed as argument from Makefile. It will be empty, if no argument passed
ARG LD_FLAGS

ADD . /kyvernopre
WORKDIR /kyvernopre

RUN CGO_ENABLED=0 GOOS=linux go build -o /output/kyvernopre -ldflags="${LD_FLAGS}" -v ./cmd/initContainer/

## Packaging stage
FROM alpine:latest

LABEL maintainer="Kyverno"

COPY --from=builder /output/kyvernopre /

RUN addgroup -S kyverno && adduser -S -G kyverno 1001
USER 1001

ENTRYPOINT ["./kyvernopre"]
