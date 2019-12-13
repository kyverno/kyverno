## MinIO Monitoring Guide

MinIO server exposes monitoring data over endpoints. Monitoring tools can pick the data from these endpoints. This document lists the monitoring endpoints and relevant documentation.

### Healthcheck Probe

MinIO server has two healthcheck related un-authenticated endpoints, a liveness probe to indicate if server is working fine and a readiness probe to indicate if server is not accepting connections due to heavy load.

- Liveness probe available at `/minio/health/live`
- Readiness probe available at `/minio/health/ready`

Read more on how to use these endpoints in [MinIO healthcheck guide](https://github.com/minio/minio/blob/master/docs/metrics/healthcheck/README.md).

### Prometheus Probe

MinIO server exposes Prometheus compatible data on a single endpoint. By default, the endpoint is authenticated.

- Prometheus data available at `/minio/prometheus/metrics`

To use this endpoint, setup Prometheus to scrape data from this endpoint. Read more on how to configure and use Prometheus to monitor MinIO server in [How to monitor MinIO server with Prometheus](https://github.com/minio/cookbook/blob/master/docs/how-to-monitor-minio-with-prometheus.md).
