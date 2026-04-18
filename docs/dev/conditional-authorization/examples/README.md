# AuthorizingPolicy Examples

These examples use the current AuthorizingPolicy CEL runtime that Kyverno exposes through `/authz/subjectaccessreview` and `/authz/conditions`.



## Sample Catalog

### 1. tenant-governance-breakglass.yaml

This sample demonstrates tenant governance and conditional escalation.

- Tenant-A developers get read-only access to Tenant-A workloads.
- Platform admins can read tenant workloads.
- Platform-admin write operations become `Conditional` and return condition IDs for break-glass style processing.

### 2. role-based-pod-access.yaml

This sample demonstrates:

- Policy-level and rule-level match conditions.
- Subresource-aware protection for `pods/exec` in production namespaces.
- Decisive deny for destructive production pod deletes.
- Conditional allow/deny signaling for incident-command style access.

### 3. api-surface-control.yaml

This sample shows API-surface authorization for non-resource requests.

- Health endpoints are open to authenticated users.
- `/metrics` access is scoped to observability groups.
- Debug endpoints (`/debug/pprof`) are conditional and require break-glass style handling.
- Unknown non-resource paths default to deny.

### 4. payload-aware-conditional.yaml

This sample demonstrates condition-phase evaluation against labels and object payload fields.

- Uses `request.resourceLabels` and `request.oldResourceLabels` derived from object metadata.
- Uses `request.object.spec` fields to evaluate risk of a deployment change.
- Models allow/deny outcomes for low-risk vs high-risk rollout updates.

## Manual Testing

Apply an example policy and send a `SubjectAccessReview` to the Kyverno endpoint:

Install on a Kind cluster (use `kind create cluster` if you do not have a cluster):

```bash
make kind-deploy-all
```

Apply sample policy:

```sh
kubectl apply -f docs/dev/conditional-authorization/examples/tenant-governance-breakglass.yaml
```

Start port-forward:

```sh
kubectl -n kyverno port-forward svc/kyverno-svc 9443:443
```

Send a SAR (adjust the user, groups, and namespace to match your deployed policy):

```bash
curl -sk https://127.0.0.1:9443/authz/subjectaccessreview \
    -H 'Content-Type: application/json' \
    --data-binary @- <<'EOF'
{
    "apiVersion": "authorization.k8s.io/v1",
    "kind": "SubjectAccessReview",
    "spec": {
        "user": "alice",
        "groups": ["tenant-a-developers"],
        "resourceAttributes": {
            "verb": "get",
            "resource": "pods",
            "namespace": "tenant-a-dev"
        }
    }
}
EOF
```

For `Conditional` policies, use `/authz/conditions` with an `AuthorizationConditionsReview` payload to obtain a concrete decision.

If the SAR request omits conditional mode support, Kyverno intentionally folds conditional rule outcomes back into a concrete deny or no-opinion response instead of returning `status.conditionSetChain`.

### Example: Conditional response from SAR endpoint

```bash
curl -sk https://127.0.0.1:9443/authz/subjectaccessreview \
    -H 'Content-Type: application/json' \
    --data-binary @- <<'EOF'
    {
        "apiVersion": "authorization.k8s.io/v1",
        "kind": "SubjectAccessReview",
        "spec": {
            "conditionalAuthorization": {
                "mode": "HumanReadable"
            },
            "user": "platform-admin-1",
            "groups": ["platform-admins"],
            "resourceAttributes": {
                "verb": "update",
                "resource": "deployments",
                "namespace": "tenant-a-prod"
            }
        }
    }
EOF
```

Expected behavior:

- The SAR result is conditional (`status.conditionSetChain` present).
- `allow-breakglass-admin-writes` and `deny-non-breakglass-admin-writes` conditions are returned.
- This only happens when `spec.conditionalAuthorization.mode` is present on the SAR request.

### Example: Non-resource API surface control

```bash
curl -sk https://127.0.0.1:9443/authz/subjectaccessreview \
    -H 'Content-Type: application/json' \
    --data-binary @- <<'EOF'
    {
        "apiVersion": "authorization.k8s.io/v1",
        "kind": "SubjectAccessReview",
        "spec": {
            "user": "alice",
            "groups": ["system:authenticated"],
            "nonResourceAttributes": {
                "verb": "get",
                "path": "/livez"
            }
        }
    }
EOF
```

Expected behavior:

- Request is allowed by the health endpoint rule.

### Example: Labels and payload data in conditional evaluation

```bash
curl -sk https://127.0.0.1:9443/authz/conditions \
    -H 'Content-Type: application/json' \
    --data-binary @- <<'EOF'
{
  "apiVersion": "authorization.k8s.io/v1alpha1",
  "kind": "AuthorizationConditionsReview",
  "request": {
    "conditionSetChain": [
      {
        "authorizerName": "kyverno",
        "conditionsType": "k8s.io/cel",
        "conditions": [
          {
            "id": "deny-high-scale-prod-change",
            "effect": "Deny",
            "condition": "request.resourceLabels['env'] == 'prod' && request.object.spec.replicas > 10"
          }
        ]
      }
    ],
    "spec": {"user": "alice", "groups": ["developers"], "resourceAttributes": {"verb": "update", "resource": "deployments", "namespace": "tenant-a-prod"}
    },
    "object": {
      "apiVersion": "apps/v1", "kind": "Deployment",
      "metadata": {"name": "checkout", "namespace": "tenant-a-prod", "labels": {"env": "prod"}},
      "spec": {"replicas": 12}
    },
    "oldObject": {"metadata": {"labels": {"release": "2026.04.12"}}}
  }
}
EOF
```

Expected behavior:

- `response.denied` is `true` when deny conditions evaluate true.
- `response.reason` explains the concrete condition decision.
