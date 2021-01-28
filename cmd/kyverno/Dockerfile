# Multi-stage docker build
# Build stage
FROM golang:1.14 AS builder

LABEL maintainer="Kyverno"

# LD_FLAGS is passed as argument from Makefile. It will be empty, if no argument passed
ARG LD_FLAGS

ADD . /kyverno
WORKDIR /kyverno

RUN CGO_ENABLED=0 GOOS=linux go build -o /output/kyverno -ldflags="${LD_FLAGS}" -v ./cmd/kyverno/

RUN useradd -u 10001 kyverno

# Packaging stage
FROM scratch

LABEL maintainer="Kyverno"

COPY --from=builder /output/kyverno /
COPY --from=builder /etc/passwd /etc/passwd

USER kyverno

ENTRYPOINT ["./kyverno"]
