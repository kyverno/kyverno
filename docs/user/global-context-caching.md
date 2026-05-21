---
title: Global Context Caching
description: Optimize policy execution speeds by caching cluster resources and external API data globally.
weight: 10
---

# GlobalContextEntry

A `GlobalContextEntry` is a cluster-scoped custom resource that enables Kyverno to cache Kubernetes API resources or external HTTP responses directly within its local memory pool. 

By centralizing and pre-fetching heavy datasets, multiple policies can evaluate incoming mutation, validation, or generation requests instantly via a `globalReference` variable. This system shifts data access from a reactive, synchronous lookup model to a proactive, highly parallel asynchronous tracking mechanism.

---

##  Concept & Architecture: The "Why"

Kyverno operates natively as a Kubernetes Admission Controller webhook. When a resource lifecycle event occurs (e.g., `kubectl apply`), Kyverno intercepts the request and must determine whether to allow or deny it with minimal latency.

### The Problem: Reactive Inline Lookup Bottlenecks
Traditionally, when a policy requires data outside the immediate admission review payload—such as comparing an incoming image tag against an allowed private enterprise registry list stored in a cluster `ConfigMap`—it relies on an inline `apiCall` context variable. 

Under high cluster density, if a continuous integration or continuous deployment (CI/CD) engine (e.g., Argo CD or Flux) executes a massive batch deployment of 200 microservices simultaneously:
* Kyverno is forced to instantiate **200 separate, concurrent network calls** to the Kubernetes API server or target external webhooks to retrieve identical tracking data.
* This scenario creates severe internal network latency spikes, induces heavy connection-throttling at the API server layer, and heavily balloons cluster CPU utilization, occasionally prompting admission timeouts.

### The Solution: Global Memory Caching
`GlobalContextEntry` decouples the policy execution pathway from external network dependencies entirely. 

Instead of executing real-time fetches during admission evaluation, Kyverno delegates resource tracking to an asynchronous background worker loop:
1. **Background Collection:** Kyverno queries the designated cluster resource or external target once, building a localized memory structure.
2. **Deterministic Refreshing:** Kyverno automatically triggers targeted polling cycles guided by user-configured intervals to keep the cache aligned with the cluster state.
3. **Instant Lookup Execution:** When the same batch of 200 microservices triggers policy evaluations, Kyverno serves the evaluation data directly out of local RAM cache. Network overhead drops to near 0 ms, ensuring horizontal stability at massive organizational scales.

---

## Configuration Modes

A `GlobalContextEntry` must express exactly one of two mutually exclusive validation specs:
* `kubernetesResource`: Monitors and maps native objects living within the cluster.
* `apiCall`: Polls external endpoints or performs structured internal HTTP interactions.

### 1. Kubernetes Resource Caching

Use this mode to capture internal topology data, structural metadata, or shared configuration boundaries (e.g., `ConfigMaps`, `Namespaces`, or custom resource configurations).

| Schema Field | Value Data Type | Required | Engine Validation Constraints |
| :--- | :--- | :--- | :--- |
| `group` | string | Conditional | The Kubernetes API group (e.g., `apps`). Required for non-core resources. Use an empty string `""` only for core API resources with `version: v1`. |
| `version` | string | **Yes** | The explicit API version state (e.g., `v1`, `v1beta1`). |
| `resource` | string | **Yes** | **Must be lowercased and pluralized** (e.g., use `configmaps` or `secrets`, not `ConfigMap`). |
| `namespace` | string | No | The target namespace boundaries. If omitted, Kyverno tracks across **all namespaces** globally. |

#### Syntax Example: Local ConfigMap Tracking
```yaml
apiVersion: kyverno.io/v2
kind: GlobalContextEntry
metadata:
  name: configmap-cache
spec:
  kubernetesResource:
    group: ""
    version: v1
    resource: configmaps
    namespace: default
```
### 2. External API Caching

Use this mode to extract and synchronize authorization lists, identity definitions, or operational constraints managed outside the local Kubernetes ecosystem.

| Schema Field | Default Value | Engine Validation Constraints |
| :--- | :--- | :--- |
| `refreshInterval` | `10m` | The background execution interval cadence. Evaluates standard duration tags (e.g., `30s`, `5m`, `2h`). Must be > 0s. |
| `retryLimit` | `3` | The exhaustion limit before an active sync loop drops and throws an error flag. Minimum allowable threshold is `1`. |

> ⚠️ **Method Enforcement:** If your `apiCall` profile passes a custom request payload under the `data` array parameter, the underlying HTTP schema engine enforces that the `method` parameter **must be explicitly configured as POST**.

#### Syntax Example: External Metadata Ingestion

```yaml
apiVersion: kyverno.io/v2
kind: GlobalContextEntry
metadata:
  name: corporate-teams-cache
spec:
  apiCall:
    service:
      url: "https://api.internal.corporate/v1/teams"
    refreshInterval: 5m
    retryLimit: 5
```
## Data Projections (JMESPath Slicing)

Caching large-scale external API payloads or extensive multi-namespace collections inside memory can stress system overhead. Kyverno supports `projections` using **JMESPath** so policies can work with a narrower, more focused view of that cached data. 

 Projections act as a high-performance filtering layer, transforming raw complex structures into exact key-value primitives or explicit string arrays for policy consumption. This helps simplify policy evaluation and reduce the amount of data rules need to traverse, but it does not necessarily remove the underlying cached payload.

```yaml
apiVersion: kyverno.io/v2
kind: GlobalContextEntry
metadata:
  name: my-cached-data
spec:
  apiCall:
    urlPath: "/api/v1/..." # Your API path
  projections:
    - name: my-projected-field  # This will be the name used in policies
      jmesPath: "data.someField" # The path within the API response to extract
```
### Referencing Projected Data in a Policy

### Eventual Consistency & Performance
**Eventual Consistency:** `GlobalContextEntry` caches are updated asynchronously. Policy evaluations may temporarily use slightly stale data until the background refresh cycle completes. Ensure your policy logic accounts for this potential delay in data propagation.

**Production Security:** When implementing external API calls:
* Use secure methods (like Kubernetes Secrets) for authentication; never hardcode credentials.
* Ensure endpoints are protected with valid TLS certificates.

**Comparison: Inline `apiCall` vs. `GlobalContextEntry`**
| Feature | Inline `apiCall` | `GlobalContextEntry` |
| :--- | :--- | :--- |
| **Performance** | High overhead (call-per-request) | High performance (cached data) |
| **Use Case** | Real-time, volatile data | Static or slowly changing data |
| **Scalability** | Can overwhelm API servers | Significantly reduces API load |

Once the `GlobalContextEntry` is defined, you can reference the projected field in your Kyverno policy as follows:

```yaml
context:
  - name: myData
    globalReference:
      name: my-cached-data    # Must match the 'name' in GlobalContextEntry metadata
      key: my-projected-field # Must match the 'key' defined in projections
```
---

## 🛠️ Practical Use Cases & Blueprints

### 1. Referencing Shared ConfigMap Data Across Multiple Policies

This scenario caches cluster-wide configuration mappings centrally so multiple running rules can cross-reference them without individual cluster query costs.

#### Step 1: Define the Cache entry

```yaml
apiVersion: kyverno.io/v2
kind: GlobalContextEntry
metadata:
  name: shared-config-cache
spec:
  kubernetesResource:
    group: ""
    version: v1
    resource: configmaps
    namespace: default
```
#### Step 2: Bind a Policy to the Entry

```yaml
apiVersion: kyverno.io/v2
kind: ClusterPolicy
metadata:
  name: require-configmap-via-gctx
spec:
  validationFailureAction: Enforce
  background: false
  rules:
  - name: configmap-must-exist
    match:
      any:
      - resources:
          kinds:
          - Deployment
    context:
    - name: cached_configmaps
      globalReference:
        name: shared-config-cache
        jmesPath: "[].metadata.name"
    validate:
      message: "Deployment initialization rejected. Required 'app-config' asset missing from GlobalContext cache."
      deny:
        conditions:
          any:
          - key: "app-config"
            operator: AnyNotIn
            value: "{{ cached_configmaps }}"
```
### 2. Caching Approved Container Registries (External API)

This implementation ensures cluster deployments pull strictly from vetted, enterprise-controlled image domains stored on an external inventory catalog manager.

#### Step 1: Define the Cache entry

```yaml
apiVersion: kyverno.io/v2
kind: GlobalContextEntry
metadata:
  name: approved-registries-cache
spec:
  apiCall:
    service:
      url: "https://api.corporate.internal/v1/registries"
    refreshInterval: 30m
    retryLimit: 3
```
#### Step 2: Bind a Policy to the Entry

Assuming the API returns: `{"allowed": ["internal-registry.io", "gcr.io/vetted-project"]}`

```yaml
apiVersion: kyverno.io/v2
kind: ClusterPolicy
metadata:
  name: restrict-registries-global
spec:
  validationFailureAction: Enforce
  background: false
  rules:
  - name: check-image-registry
    match:
      any:
      - resources:
          kinds:
          - Pod
    context:
    - name: allowed_registries
      globalReference:
        name: approved-registries-cache
        jmesPath: "allowed"
    validate:
      message: "The container image registry is unapproved by enterprise security."
      foreach:
      - list: "request.object.spec.containers"
        deny:
          conditions:
            any:
            - key: "{{ images.containers.\"{{element.name}}\".registry }}"
              operator: AnyNotIn
              value: "{{ allowed_registries }}"
```
### 3. Caching RBAC or Organisational Metadata

Useful for performance-heavy validation structures, like tracking dynamic team metadata roles across specific namespace boundaries.

#### Step 1: Define the Cache entry

```yaml
apiVersion: kyverno.io/v2
kind: GlobalContextEntry
metadata:
  name: rbac-team-cache
spec:
  kubernetesResource:
    group: "rbac.authorization.k8s.io"
    version: v1
    resource: clusterrolebindings
```
#### Step 2: Bind a Policy to the Entry

```yaml
apiVersion: kyverno.io/v2
kind: ClusterPolicy
metadata:
  name: validate-team-namespace-access
spec:
  validationFailureAction: Enforce
  background: false
  rules:
  - name: restrict-namespace-creation
    match:
      any:
      - resources:
          kinds:
          - Namespace
    context:
    - name: cluster_bindings
      globalReference:
        name: rbac-team-cache
        jmesPath: "[].subjects[?kind=='User'].name[]"
    validate:
      message: "User initiating namespace creation is not registered in cluster role metadata."
      deny:
        conditions:
          any:
          - key: "{{ request.userInfo.username }}"
            operator: AnyNotIn
            value: "{{ cluster_bindings }}"
```
## Troubleshooting

When working with `GlobalContextEntry`, misconfigurations typically manifest as empty context variables inside evaluations or synchronization blockages.

### Common Issues Matrix

| Symptom / Error | Root Cause | Remediation Steps |
| :--- | :--- | :--- |
| `GlobalContextEntry` creation fails with open errors | Singular configuration constraint violation. | Ensure you configured `kubernetesResource` or `apiCall`. Defining both fields simultaneously violates resource schemas. |
| Resource tracking results in completely empty arrays | Incorrect resource naming specification. | The `resource` tracking string **must be lowercased and pluralized**. For example, use `configmaps` instead of `ConfigMap`. |
| Policies return `variable evaluation error` on global contexts | Kyverno controllers lack appropriate RBAC clearance. | If tracking a custom resource definition (CRD), verify Kyverno's ClusterRole possesses `get`, `list`, and `watch` permissions for that specific API group. |
| External API caching loops fail continuously | Incorrect configuration of custom data variables. | If supplying a payload request template under an `apiCall` property, you must explicitly change the request `method` field to `POST`. |