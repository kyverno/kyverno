#########
# BUILD #
#########
FROM golang:1.23 AS builder
ARG TARGETPLATFORM

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/tools/webhook-cleanup ./cmd/tools/webhook-cleanup

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o webhook-cleanup ./cmd/tools/webhook-cleanup

########
# DISTROLESS IMAGE
########
FROM gcr.io/distroless/static:nonroot

# Copy the binary from builder
COPY --from=builder /app/webhook-cleanup /webhook-cleanup

USER 65532:65532

ENTRYPOINT ["/webhook-cleanup"]
