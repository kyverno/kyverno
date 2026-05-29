# Kyverno Performance Testing on Akamai

This guide sets up a dedicated, repeatable load-testing environment on Akamai Cloud (Linode) bare-metal instances to benchmark admission review latency for `ValidatingPolicy` and `MutatingPolicy` (CEL-based policy types, `policies.kyverno.io/v1beta1`).

## Why Akamai

GitHub Actions `ubuntu-latest` runners (4 vCPU / 16 GB) distort high-concurrency latency numbers. Akamai Dedicated instances give consistent, bare-metal-class compute that matches the test environment used for the published benchmarks at [kyverno.io — Scaling Kyverno](https://kyverno.io/docs/installation/scaling/).

## Topology

| Instance label | Linode type | vCPU | RAM | Purpose |
|----------------|-------------|------|-----|---------|
| `kyverno-perf-cluster` | `g6-dedicated-64` | 32 | 64 GB | KinD cluster: control plane + workers + KWOK + Prometheus |
| `kyverno-perf-loader` | `g6-dedicated-16` | 8 | 16 GB | k6 load generator |

Both instances join the `kyverno-perf-vlan` VLAN (10.0.0.0/24) so load traffic stays on a private LAN interface and avoids Akamai egress charges.

---

## Prerequisites

- `linode-cli` authenticated: run `linode-cli configure`
- `jq` installed locally
- SSH public key in `$LINODE_SSH_KEY`

---

## Step 1 — Provision Instances

```sh
export LINODE_SSH_KEY="$(cat ~/.ssh/id_ed25519.pub)"
./docs/perf-testing/provision/akamai-provision.sh
```

This creates both Linodes and waits for them to reach `running` status. It prints SSH instructions and stores instance IDs in `/tmp/kyverno-perf-instance-ids` for teardown.

Optional: override the region:
```sh
REGION=us-west ./docs/perf-testing/provision/akamai-provision.sh
```

---

## Step 2 — Bootstrap the Cluster Node

```sh
# From your local machine (replace <cluster-ip> with the printed public IP)
ssh root@<cluster-ip> 'bash -s' < docs/perf-testing/provision/akamai-setup-cluster.sh
```

This installs Docker, kind, helm, kwok, creates a 3-worker KinD cluster, deploys Prometheus, and creates a KWOK fake node. Expect ~10 minutes.

---

## Step 3 — Bootstrap the Loader Node

```sh
ssh root@<loader-ip> 'bash -s' < docs/perf-testing/provision/akamai-setup-loader.sh
```

This installs Go, kubectl, and builds k6 with the `xk6-kubernetes` extension.

---

## Step 4 — Copy Kubeconfig to Loader

```sh
scp root@<cluster-ip>:/root/.kube/config /tmp/kyverno-perf-kubeconfig

# Point the server at the VLAN IP (10.0.0.10) so loader traffic stays on LAN
sed -i 's|server: https://127.0.0.1:[0-9]*|server: https://10.0.0.10:6443|g' \
    /tmp/kyverno-perf-kubeconfig

scp /tmp/kyverno-perf-kubeconfig root@<loader-ip>:/root/.kube/config
```

---

## Step 5 — Install Kyverno

On the **cluster node**:

```sh
helm repo add kyverno https://kyverno.github.io/kyverno/
helm repo update

# 1-replica run (adjust --set admissionController.replicas for 3-replica runs)
helm upgrade --install kyverno kyverno/kyverno -n kyverno \
  --create-namespace \
  --set admissionController.replicas=1 \
  --set admissionController.serviceMonitor.enabled=true \
  --set reportsController.serviceMonitor.enabled=true \
  --set "features.omitEvents.eventTypes={PolicyApplied,PolicySkipped,PolicyViolation,PolicyError}" \
  --values charts/kyverno/ci/monitoring-values.yaml \
  --wait --timeout 10m
```

---

## Step 6 — Apply Test Policies

Apply **one policy set at a time**; delete between scenario runs to avoid cross-contamination.

```sh
# ValidatingPolicy: single CEL check
kubectl apply -f docs/perf-testing/v1.18.1/policies/vpol-simple.yaml

# ValidatingPolicy: 3-field + variable
kubectl apply -f docs/perf-testing/v1.18.1/policies/vpol-moderate.yaml

# ValidatingPolicy: 6-field + 3 variables (security context checks)
kubectl apply -f docs/perf-testing/v1.18.1/policies/vpol-complex.yaml

# ValidatingPolicy: 16-policy PSS baseline (apples-to-apples with published benchmarks)
kubectl apply -f docs/perf-testing/v1.18.1/policies/vpol-pss-baseline.yaml

# MutatingPolicy: add single label via JSONPatch
kubectl apply -f docs/perf-testing/v1.18.1/policies/mpol-simple.yaml

# MutatingPolicy: 3 labels + annotation (2 mutation blocks)
kubectl apply -f docs/perf-testing/v1.18.1/policies/mpol-moderate.yaml

# MutatingPolicy: ApplyConfiguration sidecar inject
kubectl apply -f docs/perf-testing/v1.18.1/policies/mpol-complex.yaml
```

To delete all test policies:
```sh
kubectl delete validatingpolicies -l app.kubernetes.io/part-of=kyverno-perf-test 2>/dev/null || \
kubectl delete -f docs/perf-testing/v1.18.1/policies/ --ignore-not-found
```

---

## Step 7 — Run k6 Tests

Expose Prometheus from the cluster node (keep this running in a background shell):

```sh
# On the cluster node
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090 --address 0.0.0.0 &
```

On the **loader node**, run k6 against a single scenario. Use the same flags as the existing CI workflow:

```sh
# Example: ValidatingPolicy simple, 200 VUs, 10000 iterations
K6_WEB_DASHBOARD_PERIOD=5s \
K6_WEB_DASHBOARD_EXPORT=k6-vpol-simple-200vu.html \
K6_PROMETHEUS_RW_TREND_STATS="avg,min,med,max,p(90),p(95),p(99)" \
k6 run \
  --vus 200 --iterations 10000 \
  --no-connection-reuse \
  --no-vu-connection-reuse \
  --no-usage-report \
  --quiet \
  --summary-mode full \
  --summary-export k6-summary-vpol-simple-200vu.json \
  --summary-trend-stats "avg,min,med,max,p(90),p(95),p(99)" \
  --new-machine-readable-summary \
  --out web-dashboard \
  --out "experimental-prometheus-rw=http://10.0.0.10:9090/api/v1/write" \
  --out "json=k6-vpol-simple-200vu.json" \
  docs/perf-testing/v1.18.1/scripts/k6/vpol-script.js
```

For MutatingPolicy scenarios, replace `vpol-script.js` with `mpol-script.js`.

### Running the Full Matrix

The full test matrix is defined in `docs/perf-testing/v1.18.1/matrix.json` (54 runs across 17 scenarios × 4 load levels, minus baseline-only rows). Use `jq` to drive a loop:

```sh
jq -c '.[]' docs/perf-testing/v1.18.1/matrix.json | while read -r entry; do
  scenario=$(echo "$entry" | jq -r '.scenario')
  replicas=$(echo "$entry" | jq -r '.replicas')
  vus=$(echo "$entry"     | jq -r '.concurrent_connections')
  iters=$(echo "$entry"   | jq -r '.total_iterations')
  script=$(echo "$entry"  | jq -r '.k6_script')

  echo "==> Running $scenario VUs=$vus iters=$iters replicas=$replicas"

  k6 run \
    --vus "$vus" --iterations "$iters" \
    --no-connection-reuse --no-vu-connection-reuse \
    --no-usage-report --quiet \
    --summary-mode full \
    --summary-export "summaries/${scenario}-${vus}vu-summary.json" \
    --summary-trend-stats "avg,min,med,max,p(90),p(95),p(99)" \
    --new-machine-readable-summary \
    --out "json=summaries/${scenario}-${vus}vu.json" \
    --out "experimental-prometheus-rw=http://10.0.0.10:9090/api/v1/write" \
    "$script"
done
```

---

## Step 8 — Collect and Aggregate Results

On the **cluster node**, after each run:

```sh
# Kyverno pod CPU + memory (from Prometheus)
./scripts/kyverno-pods-resources-report.sh http://localhost:9090

# etcd size (for reports controller scenarios)
./scripts/etcd-storage-report.sh http://localhost:9090
```

Aggregate k6 summaries into a single markdown table (same script used by CI):

```sh
mkdir -p summaries/vpol-simple summaries/vpol-moderate  # etc.
# move per-scenario JSON files into subdirs by name, then:
./scripts/k6-aggregate-summary.sh summaries \
  baseline vpol-simple vpol-moderate vpol-complex vpol-pss-baseline \
  mpol-simple mpol-moderate mpol-complex combined
```

---

## What to Look For

### Admission Latency by Policy Type (p99 target reference)

| Scenario | Published baseline (ClusterPolicy PSS, 1r, 500VU) | Expected direction |
|----------|--------------------------------------------------|--------------------|
| `vpol-simple-1r` (500VU) | — (new data) | < PSS ClusterPolicy p99 (133ms) |
| `vpol-pss-1r` (500VU) | 133.67ms avg, 1.63s max | apples-to-apples; should be ≈ same |
| `mpol-simple-1r` (500VU) | — (new data) | |
| `combined-1r` (500VU) | — (new data) | expect additive overhead |

**Alert threshold validation:** The current `KyvernoAdmissionHighLatency` PrometheusRule fires at `p99 > 1s`. If any scenario's steady-state p99 (not transient max) approaches 1s at moderate load (≤ 200 VU), the threshold needs adjustment.

### Signals That Need Follow-Up

- p99 > 500ms at ≤ 50 VU on a 3-replica deployment → possible webhook bottleneck
- p99 that grows super-linearly with policy count (simple → moderate → complex) → CEL evaluation scaling issue
- Error rate > 0% at ≤ 200 VU → admission webhook timeout (check `failurePolicy`)
- Kyverno CPU > 2000m at 50 VU on 1 replica → resource request tuning needed

---

## Step 9 — Teardown

```sh
./docs/perf-testing/provision/akamai-provision.sh teardown
```

This deletes both Linode instances using the IDs saved during provisioning.