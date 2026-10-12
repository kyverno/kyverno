# Build environment for the Anthropic OSS Scanner (https://github.com/anthropics/oss-scanner).
# The scanner builds this file with the repository as the build context. Network access is
# available during this build only; it is removed before the security audit starts.
FROM golang:1.27-bookworm

ENV DEBIAN_FRONTEND=noninteractive \
    CGO_ENABLED=0 \
    GOTOOLCHAIN=auto

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates git make \
    && rm -rf /var/lib/apt/lists/*

# scanner contract: the checkout lives inside the image, at /src
COPY . /src
WORKDIR /src

# Fetch every dependency now, while the network is still available.
RUN go mod download

# Build all Kyverno binaries with debug info (no -s -w) so findings are easier to reproduce.
RUN mkdir -p /src/bin \
    && go build -o /src/bin/kyverno ./cmd/kyverno \
    && go build -o /src/bin/kyvernopre ./cmd/kyverno-init \
    && go build -o /src/bin/background-controller ./cmd/background-controller \
    && go build -o /src/bin/cleanup-controller ./cmd/cleanup-controller \
    && go build -o /src/bin/reports-controller ./cmd/reports-controller \
    && go build -o /src/bin/readiness-checker ./cmd/readiness-checker \
    && go build -o /src/bin/kubectl-kyverno ./cmd/cli/kubectl-kyverno

# Run the policy engine unit tests as a sanity check (this also warms the build cache so tests can be
# re-run offline during the audit). A failing test must not abort the build.
RUN go test -count=1 ./pkg/engine/... ./pkg/cel/... ./pkg/validation/... \
    || echo "WARNING: unit tests reported failures"
