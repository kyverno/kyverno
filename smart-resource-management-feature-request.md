# Smart Resource Management for Containers

## Summary
Add support for intelligent default resource management in Kyverno that automatically assigns appropriate resource requests and limits based on container profiles, historical usage, and environment contexts.

## Motivation
In Kubernetes environments, improperly configured resource requirements can lead to either resource starvation (when limits are too low) or inefficient resource utilization (when requests are too high). While Kyverno already has policies for validating resource requirements and generating ResourceQuotas/LimitRanges at the namespace level, it lacks the ability to automatically assign appropriate resource configurations based on container workload types and historical usage.

## Goals
1. Provide automatic resource assignment for containers based on predefined profiles (e.g., database, web server, batch job)
2. Support environment-specific configurations (dev, test, prod) with different resource allocation strategies
3. Allow for monitoring and historical analysis of resource usage to improve future assignments
4. Enable cluster administrators to define custom profiles and rules for resource assignment

## Non-Goals
1. Replace manual resource configuration for specialized workloads
2. Implement real-time resource adjustment (this would be handled by VPA)

## Proposal
Create a new feature in Kyverno that:

1. Extends the mutation capability to automatically add appropriate resource requests and limits to containers
2. Provides a ConfigMap-based configuration for defining container profiles and their resource requirements
3. Implements a basic monitoring system to track resource usage for future recommendations

### User Experience

Cluster administrators would:
1. Deploy a ConfigMap with container profiles and resource configurations
2. Create a Kyverno policy that references these profiles

Example policy:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: smart-resource-management
spec:
  rules:
  - name: apply-resource-profiles
    match:
      any:
      - resources:
          kinds:
          - Pod
    mutate:
      patchStrategicMerge:
        spec:
          containers:
          - (name): \"*\"
            resources:
              # Apply smart resource management based on container profile
              # determined by image, labels, or annotations
              smartResources:
                profile: \"{{container.image | extractProfile}}\"
                environment: \"{{request.object.metadata.namespace | getEnvironmentType}}\"
```

### Implementation Details

1. Add a new mutation type `smartResources` that can analyze container attributes and apply appropriate resource settings
2. Implement functions to extract profiles from container images or other attributes
3. Create a ConfigMap schema for defining profiles and their resource requirements
4. Add monitoring capabilities to track actual resource usage

## References
Related issues:
- #12235 (focuses on Helm chart resources, not automatic policy for all workloads)

Similar features in other tools:
- Vertical Pod Autoscaler (different approach, adjusts resources after deployment)
- Various cluster monitoring solutions

## Test Scenarios

The following test scenarios demonstrate how the smart resource management feature would work.

### Test Case 1: Web Server Profile Detection and Application

**Input: Deployment without resources defined**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
```

**Expected Output: Resources added based on profile**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "200m"
            memory: "256Mi"
```

### Test Case 2: Database Container Profile

**Input: StatefulSet without resources defined**

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres-db
spec:
  serviceName: "postgres"
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:13
        env:
        - name: POSTGRES_PASSWORD
          value: mysecretpassword
        ports:
        - containerPort: 5432
```

**Expected Output: Resources added based on database profile**

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres-db
spec:
  serviceName: "postgres"
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:13
        env:
        - name: POSTGRES_PASSWORD
          value: mysecretpassword
        ports:
        - containerPort: 5432
        resources:
          requests:
            cpu: "500m"
            memory: "1Gi"
          limits:
            cpu: "1"
            memory: "2Gi"
```

### Test Case 3: Environment-Specific Resource Allocation

**Configuration ConfigMap**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: resource-profiles
  namespace: kyverno
data:
  profiles: |
    {
      "web": {
        "dev": {
          "requests": {
            "cpu": "50m",
            "memory": "64Mi"
          },
          "limits": {
            "cpu": "100m",
            "memory": "128Mi"
          }
        },
        "prod": {
          "requests": {
            "cpu": "200m",
            "memory": "256Mi"
          },
          "limits": {
            "cpu": "500m",
            "memory": "512Mi"
          }
        }
      }
    }
```

**Input Pod in dev namespace:**
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: web-frontend
  namespace: dev
spec:
  containers:
  - name: frontend
    image: nginx:1.14.2
```

**Expected output in dev namespace:**
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: web-frontend
  namespace: dev
spec:
  containers:
  - name: frontend
    image: nginx:1.14.2
    resources:
      requests:
        cpu: "50m"
        memory: "64Mi"
      limits:
        cpu: "100m" 
        memory: "128Mi"
```

### Test Case 4: Respecting Existing Resource Settings

**Input Pod with partial resources:**
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: web-frontend
  namespace: dev
spec:
  containers:
  - name: frontend
    image: nginx:1.14.2
    resources:
      requests:
        memory: "128Mi"
```

**Expected output (only add missing resources):**
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: web-frontend
  namespace: dev
spec:
  containers:
  - name: frontend
    image: nginx:1.14.2
    resources:
      requests:
        memory: "128Mi"  # Original value preserved
        cpu: "50m"       # Added missing value
      limits:
        cpu: "100m"      # Added missing value
        memory: "256Mi"  # Added missing value
```

### Test Execution Strategy

The implementation should include Chainsaw tests similar to existing Kyverno tests. For example:

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: test-smart-resource-profile-web
spec:
  steps:
  - name: create-profile-config
    apply:
      file: config-resource-profile.yaml
  - name: create-web-policy
    apply: 
      file: policy-smart-resources.yaml
  - name: deploy-app-without-resources
    apply:
      file: deployment-without-resources.yaml
  - name: check-resources-applied
    assert:
      file: expected-deployment-with-resources.yaml
```

#### Test File Structure

Following Kyverno's testing conventions, the tests would be organized as follows:

```
test/conformance/chainsaw/smart-resources/
├── web-profile
│   ├── test.yaml              # Test definition
│   └── data
│       ├── config-resource-profile.yaml
│       ├── deployment-without-resources.yaml
│       ├── expected-deployment-with-resources.yaml
│       └── policy-smart-resources.yaml
├── database-profile
│   ├── test.yaml
│   └── data
│       ├── config-resource-profile.yaml
│       ├── statefulset-without-resources.yaml
│       ├── expected-statefulset-with-resources.yaml
│       └── policy-smart-resources.yaml
└── environment-specific
    ├── test.yaml
    └── data
        ├── config-resource-profile.yaml
        ├── pod-dev-namespace.yaml
        ├── pod-prod-namespace.yaml
        ├── expected-pod-dev-resources.yaml
        ├── expected-pod-prod-resources.yaml
        └── policy-smart-resources.yaml
```

Example `test.yaml` for the web-profile test:

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: test-smart-resource-profile-web
spec:
  steps:
  - name: create-namespace
    apply:
      file: data/namespace.yaml
  - name: create-profile-config
    apply:
      file: data/config-resource-profile.yaml
  - name: create-policy
    apply: 
      file: data/policy-smart-resources.yaml
  - name: deploy-app-without-resources
    apply:
      file: data/deployment-without-resources.yaml
  - name: check-resources-applied
    assert:
      file: data/expected-deployment-with-resources.yaml
```

Example `namespace.yaml`:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: smart-resources-test
```

Example `policy-smart-resources.yaml`:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: smart-resource-management
spec:
  rules:
  - name: apply-resource-profiles
    match:
      any:
      - resources:
          kinds:
          - Pod
          - Deployment
          - StatefulSet
    mutate:
      patchStrategicMerge:
        spec:
          template:
            spec:
              containers:
              - (name): "*"
                resources:
                  smartResources:
                    profile: "{{images.containers.*.name | extractProfile}}"
                    environment: "{{request.object.metadata.namespace | getEnvironmentType}}"
```

## Timeline
TBD
