# Kyverno Performance Testing

## Published Benchmarks and Performance Baselines

Performance benchmarks for Kyverno are published at:

**[kyverno.io — Scaling Kyverno](https://kyverno.io/docs/installation/scaling/)**

The published data includes admission controller latency (p95, p99) under varying load conditions (50–500 virtual users, single and multi-replica deployments). These figures serve as the project's informal performance baselines.

Kyverno does not publish formal SLO commitments. The benchmarks on the scaling page are observational results from controlled test runs and are intended as guidance for capacity planning. Operators should conduct load testing in their own environments using the harness described in each version's subdirectory.

---

## Benchmark Results by Version

| Version | Results | Policy API | Hardware |
| ------- | ------- | ---------- | -------- |
| v1.18.1 | [results/summary.md](v1.18.1/results/summary.md) | `policies.kyverno.io/v1beta1` ValidatingPolicy + MutatingPolicy (CEL) | Akamai Dedicated g6-dedicated-32 (32 vCPU / 64 GB), KinD + KWOK |

Each version subdirectory contains:
- `README.md` — step-by-step setup and run guide
- `policies/` — test policy YAML files
- `scripts/k6/` — k6 load test scripts
- `matrix.json` — full test matrix (scenarios × VU levels)
- `results/` — aggregated results and raw k6 JSON

---

## Shared Infrastructure Scripts

Provisioning scripts for Akamai Dedicated instances are in [`provision/`](provision/) and are shared across versions. Override the Kyverno Helm chart version via the `KYVERNO_VERSION` environment variable:

```sh
# Bootstrap cluster node (defaults to Helm chart 3.8.1 / Kyverno v1.18.1)
ssh root@<cluster-ip> 'bash -s' < docs/perf-testing/provision/akamai-setup-cluster.sh

# Override for a newer release
KYVERNO_VERSION=3.9.0 ssh root@<cluster-ip> 'bash -s' < docs/perf-testing/provision/akamai-setup-cluster.sh
```

| Script | Purpose |
| ------ | ------- |
| [`provision/akamai-provision.sh`](provision/akamai-provision.sh) | Create both Akamai Linodes + VLAN via `linode-cli` |
| [`provision/akamai-setup-cluster.sh`](provision/akamai-setup-cluster.sh) | Bootstrap cluster node: Docker, KinD, Helm, KWOK, Prometheus |
| [`provision/akamai-setup-loader.sh`](provision/akamai-setup-loader.sh) | Bootstrap loader node: k6 + xk6-kubernetes |

---

## Load Testing in CI

A load-testing workflow (`.github/workflows/tests-k6.yaml`) runs automated load tests as part of the release process against the `ClusterPolicy` (`kyverno.io/v1`) API. Results feed into the benchmarks published on the scaling page above.

---

## Reports Controller Testing (legacy)

The scripts below were written for Kyverno 1.12 with k3d and test the reports controller under high pod/policyreport counts. They remain usable but are not actively maintained.

- `deployment.sh`, `pod.sh`, `node.sh`, `kwok.sh` — workload and node creation helpers
- `size.sh`, `setupetcd.sh` — etcd size measurement
- See the older k3d-based instructions in git history for full context.
