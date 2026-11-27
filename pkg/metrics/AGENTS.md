# AGENTS.md — pkg/metrics

Package `pkg/metrics` manages OpenTelemetry instrumentation, metric configuration management, and exporter setup for Kyverno components. See [ARCHITECTURE.md](../../ARCHITECTURE.md) for how metrics fit into Kyverno's runtime architecture.

## Exporter Selection: Prometheus vs. OTLP gRPC

Kyverno supports two metric export configurations via `--otelConfig`:

1. **`prometheus` (default)**:
   - Sets up an in-process OpenTelemetry Prometheus exporter and `http.ServeMux`.
   - Exposes `/metrics` via `promhttp.Handler()`.
   - Hosts controller health probe endpoints: `/health/liveness`, `/health/readiness`, and `/healthz`.
   - If metrics export is disabled via `--disableMetrics=true`, the `/metrics` handler is omitted while the health probe endpoints remain active.

2. **`grpc`**:
   - Sets up an OTLP gRPC exporter that pushes metrics to an external OpenTelemetry collector (`--otelCollector`).
   - Does **not** start an in-process HTTP server.
   - Any health probes expecting an HTTP listener on the metrics port will not be reachable when `otelConfig` is `grpc`. Helm charts conditionally omit default probes when `metering.config: grpc` is set.

## HTTP and TLS Server Lifecycle

The HTTP/TLS listener lifecycle for Prometheus metrics is orchestrated in `cmd/internal/metrics.go`:

- **Address & Port**: Listens on `metricsAddr` (derived from `--metricsPort`, default `8000`).
- **TLS Configuration**: When `--metricsTLSSecretName` and `--metricsCASecretName` are provided, a certificate controller (`kyvernotls.NewCertRenewer`) creates and dynamically renews TLS secrets. The metrics server serves HTTPS using `tlsConfig.GetCertificate` to dynamically reload updated TLS certificates.
- **Probe Scheme**: In environments where `metering.secure: true` is configured, Kubernetes probes targeting the metrics port must use `scheme: HTTPS` (kubelet skips certificate verification for probes by default). Plain `scheme: HTTP` requests will fail the TLS handshake.
