# Multi-stage docker build
# Build stage
ARG USER=kyverno
ARG UID=1000
ARG GID=1000

FROM golang:1.14 AS builder

LABEL maintainer="Kyverno"

# LD_FLAGS is passed as argument from Makefile. It will be empty, if no argument passed
ARG LD_FLAGS
ARG USER
ARG UID

ADD . /kyverno
WORKDIR /kyverno

RUN CGO_ENABLED=0 GOOS=linux go build -o /output/kyverno -ldflags="${LD_FLAGS}" -v ./cmd/cli/kubectl-kyverno/

RUN useradd -m $USER --uid=${UID}

# Packaging stage
FROM scratch

LABEL maintainer="Kyverno"

ARG UID
ARG GID

COPY --from=builder /output/kyverno /

ENTRYPOINT ["./kyverno"]

USER ${UID}:${GID}
