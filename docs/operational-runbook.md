# Kyverno Operational Runbook

This document covers how to safely disable or limit Kyverno during a production incident, and how to recover afterward.

For day-to-day operations see the [installation guide](https://kyverno.io/docs/installation/) and [monitoring docs](https://kyverno.io/docs/monitoring/).

For webhook certificate issuance, rotation, and cert-manager integration see the [Certificate Management guide](https://kyverno.io/docs/installation/customization/#certificate-management).

---

## Table of Contents

- [Incident Decision Tree](#incident-decision-tree)
- [Disable Patterns](#disable-patterns)
  - [1. Set failurePolicy to Ignore (least disruptive)](#1-set-failurepolicy-to-ignore-least-disruptive)
  - [2. Disable a single policy](#2-disable-a-single-policy)
  - [3. Exempt a namespace](#3-exempt-a-namespace)
  - [4. Create a PolicyException](#4-create-a-policyexception)
  - [5. Scale the admission controller to zero (most disruptive)](#5-scale-the-admission-controller-to-zero-most-disruptive)
- [Recovery Sequence](#recovery-sequence)
- [What to Capture for Post-Incident Review](#what-to-capture-for-post-incident-review)

---

## Incident Decision Tree

```
Is Kyverno blocking or causing issues?
│
├── Yes: Is only one policy/namespace affected?
│   ├── Yes ──▶ Option 2 or 3 (disable one policy or exempt namespace)
│   └── No ──▶ Is it a cert/webhook connectivity issue?
│               ├── Yes ──▶ Option 1 (failurePolicy: Ignore) while you fix certs
│               └── No ──▶ Option 5 (scale to zero) as last resort
│
└── No: Are you doing maintenance that requires bypassing policies temporarily?
    ├── Single resource ──▶ Option 4 (PolicyException)
    └── Single namespace ──▶ Option 3 (namespace label)
```

---

## Disable Patterns

### 1. Set failurePolicy to Ignore (least disruptive)

**What it does:** Tells the API server to allow requests even if Kyverno is unreachable or returns an error. Kyverno keeps running; background scans continue.

**Blast radius:** All in-scope admission enforcement is bypassed if Kyverno is unavailable. Mutations and validations still run when Kyverno is healthy.

```bash
# List all Kyverno webhooks
kubectl get validatingwebhookconfigurations,mutatingwebhookconfigurations \
  -o name | grep kyverno

# Patch failurePolicy on a ValidatingWebhookConfiguration
kubectl patch validatingwebhookconfiguration kyverno-resource-validating-webhook-cfg \
  --type='json' \
  -p='[{"op":"replace","path":"/webhooks/0/failurePolicy","value":"Ignore"}]'

# Patch the mutating webhook similarly
kubectl patch mutatingwebhookconfiguration kyverno-resource-mutating-webhook-cfg \
  --type='json' \
  -p='[{"op":"replace","path":"/webhooks/0/failurePolicy","value":"Ignore"}]'
```

**Restore:** Patch back to `Fail` once the issue is resolved.

---

### 2. Disable a single policy

**What it does:** Disables admission enforcement for a specific policy without deleting it.

**Blast radius:** Targeted — only that policy stops enforcing at admission time.

**For `ClusterPolicy` / `Policy` (kyverno.io):**

```bash
kubectl patch clusterpolicy <policy-name> \
  --type='merge' \
  -p='{"spec":{"admission":false}}'

# For namespaced Policy:
kubectl patch policy <policy-name> -n <namespace> \
  --type='merge' \
  -p='{"spec":{"admission":false}}'
```

**For CEL-based policies (`policies.kyverno.io`):**

```bash
# ValidatingPolicy
kubectl patch validatingpolicy <policy-name> \
  --type='merge' \
  -p='{"spec":{"evaluation":{"admission":false}}}'

# MutatingPolicy
kubectl patch mutatingpolicy <policy-name> \
  --type='merge' \
  -p='{"spec":{"evaluation":{"admission":false}}}'

# Namespaced variants use the same patch on the namespaced resource kind
```

**Restore:** Patch `spec.admission` back to `true`, or `spec.evaluation.admission` back to `true`.

---

### 3. Exempt a namespace

**What it does:** Labels a namespace so Kyverno skips all policies for resources in that namespace.

**Blast radius:** All Kyverno policies stop enforcing in that namespace.

```bash
kubectl label namespace <namespace> kyverno.io/exclude="true"
```

**Restore:**

```bash
kubectl label namespace <namespace> kyverno.io/exclude-
```

> **Note:** The exact label key depends on your Kyverno configuration (`config.webhooks` in the Kyverno ConfigMap). Verify the configured exclusion label before using.

---

### 4. Create a PolicyException

**What it does:** Grants a specific workload (by name + namespace) an exception from one or more named policies. The exception is recorded as a Kubernetes object and is auditable.

**Blast radius:** Targeted — only the named resources in the named namespace are excepted.

**For `ClusterPolicy` / `Policy` (kyverno.io):**

```yaml
apiVersion: kyverno.io/v2
kind: PolicyException
metadata:
  name: emergency-exception
  namespace: <namespace>
spec:
  exceptions:
    - policyName: <policy-name>
      ruleNames:
        - <rule-name>        # or "*" for all rules in the policy
  match:
    any:
      - resources:
          kinds:
            - Deployment
          names:
            - <deployment-name>
          namespaces:
            - <namespace>
```

**For CEL-based policies (`policies.kyverno.io`):**

```yaml
apiVersion: policies.kyverno.io/v1alpha1
kind: PolicyException
metadata:
  name: emergency-cel-exception
  namespace: <namespace>
spec:
  policyRefs:
    - name: <policy-name>
      kind: ValidatingPolicy    # or MutatingPolicy
  matchConditions:
    - name: match-target-namespace
      expression: "object.metadata.namespace == '<namespace>'"
    - name: match-target-deployment
      expression: "object.metadata.name == '<deployment-name>'"
```

```bash
kubectl apply -f emergency-exception.yaml
```

**Restore:** `kubectl delete policyexception emergency-exception -n <namespace>`
(Use the same command for CEL exceptions, substituting the CEL exception name.)

---

### 5. Scale the admission controller to zero (most disruptive)

**What it does:** Removes all Kyverno pods. With `failurePolicy: Fail` (the default), the API server will **reject all resource requests in scope** once the webhook times out. Change `failurePolicy` to `Ignore` first (Option 1) before scaling to zero, or the cluster will be unable to admit any resources.

**Blast radius:** All admission policies disabled; no validation, mutation, or image verification at admission time. Background scans also stop.

```bash
# Step 1: Set failurePolicy to Ignore first (see Option 1 above)

# Step 2: Scale down
kubectl scale deployment kyverno-admission-controller \
  -n kyverno --replicas=0
kubectl scale deployment kyverno-background-controller \
  -n kyverno --replicas=0
kubectl scale deployment kyverno-reports-controller \
  -n kyverno --replicas=0
kubectl scale deployment kyverno-cleanup-controller \
  -n kyverno --replicas=0
```

**Restore:**

```bash
kubectl scale deployment kyverno-admission-controller \
  -n kyverno --replicas=1
# Scale other controllers back up as needed, then restore failurePolicy to Fail
```

---

## Recovery Sequence

After resolving the incident, restore enforcement in reverse order:

1. **Verify Kyverno is healthy:** `kubectl get pods -n kyverno` — all pods Running/Ready.
2. **Re-enable in reverse order of what you disabled:**
   - Remove `PolicyException` objects created during the incident.
   - Remove namespace exclusion labels.
   - Re-apply deleted policies from Git.
   - Restore `failurePolicy: Fail` on webhook configurations.
3. **Run a background scan sweep** to catch any drift that occurred while enforcement was off:
   ```bash
   # Trigger an immediate background scan by restarting the background controller
   kubectl rollout restart deployment kyverno-background-controller -n kyverno
   ```
4. **Review PolicyReports** for new violations: `kubectl get policyreports -A`.
5. **Restore monitoring alerts** if they were silenced during the incident.

---

## What to Capture for Post-Incident Review

Before restoring enforcement, capture the following:

```bash
# Current state of all Kyverno webhook configurations
kubectl get validatingwebhookconfigurations,mutatingwebhookconfigurations \
  -o yaml | grep -A5 kyverno > webhook-state.yaml

# Kyverno controller logs from the incident window
kubectl logs -n kyverno -l app.kubernetes.io/part-of=kyverno \
  --since=2h > kyverno-incident-logs.txt

# Any PolicyReports generated during the incident
kubectl get policyreports,clusterpolicyreports -A -o yaml > policy-reports.yaml

# List of any PolicyExceptions created
kubectl get policyexceptions -A
```

Record in your incident document:
- Which option(s) were invoked and at what time
- What resources were created or admitted while enforcement was off
- Whether any policies were deleted (vs. temporarily disabled) — if deleted, re-apply from Git before restoring
