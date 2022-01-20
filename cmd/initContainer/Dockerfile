# Multi-stage docker build
# Build stage
FROM golang:1.17.6 AS builder

LABEL maintainer="Kyverno"

# LD_FLAGS is passed as argument from Makefile. It will be empty, if no argument passed
ARG LD_FLAGS
ARG TARGETPLATFORM

ADD . /kyverno
WORKDIR /kyverno

RUN export GOOS=$(echo ${TARGETPLATFORM} | cut -d / -f1) && \
    export GOARCH=$(echo ${TARGETPLATFORM} | cut -d / -f2)

RUN go env

RUN CGO_ENABLED=0 go build -o /output/kyvernopre -ldflags="${LD_FLAGS}" -v ./cmd/initContainer/

# Packaging stage
FROM scratch

LABEL maintainer="Kyverno"

COPY --from=builder /output/kyvernopre /
COPY --from=builder /etc/passwd /etc/passwd

USER 10001

ENTRYPOINT ["./kyvernopre"]
