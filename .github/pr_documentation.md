## PR Documentation

In order to assist the Kyverno maintainers of both the software and documentation as well as to provide clarity to PR reviewers, any PRs which result in new or different behavior exposed to users must be captured in the documentation. In order to ensure these changes do not fall by the wayside, follow this guide if your PR results in new or changed behavior to Kyverno which impacts users. Examples of changes which fall under this definition:

* Adding a command or flags to the Kyverno CLI
* Adding API lookup capabilities
* Changing schema definitions
* Adding multi-line YAML lookups
* Other functionality that users can "touch"

Examples of changes which are exempt:

* Bug fixes
* Logging level or message changes
* Test cases
* Other changes which are internal to the code base

If you are unsure what type your PR falls under, please either start a thread on the [Kyverno Slack channel](https://kubernetes.slack.com/) or a [discussion](https://github.com/kyverno/kyverno/discussions).

## Story Process

If your PR does result in new or altered behavior, under the Proposed Changes section of the PR, please describe the following:

1. What was Kyverno's behavior before your PR
2. What does this PR do
3. What is the resulting behavior after your PR

### Example

1. Prior to this PR, ConfigMaps had to be created with JSON-encoded data such as:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: roles-dictionary
  namespace: default
data:
  allowed-roles: "[\"cluster-admin\", \"cluster-operator\", \"tenant-admin\"]"
```

2. This PR adds the ability to specify string array values in ConfigMap resources as multi-line YAML (block scalars) as opposed to JSON-encoded data.

3. After this PR, ConfigMaps can now be created as follows:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
    name: roles-dictionary
    namespace: default
data:
  allowed-roles: |-
    cluster-admin
    cluster-operator
    tenant-admin
```

## Proof Manifests

To assist the docs maintainers in updating the documentation (if you have not done so yourself) and code maintainers/community to quickly understand and test your PR, please provide YAML manifests which help them "prove" your changes.

### Example

To test this PR's behavior, create `cm.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: animals
  namespace: default
data:
  animals: |-
    snake
    bear
    cat
    dog
```

Create `cpol.yaml`:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: cm-array-example
spec:
  validationFailureAction: enforce
  background: false
  rules:
  - name: validate-role-annotation
    context:
      - name: animals
        configMap:
          name: animals
          namespace: default
    match:
      resources:
        kinds:
        - Deployment
    validate:
      message: "The animal {{ request.object.metadata.labels.animal }} is not in the allowed list of animals: {{ animals.data.animals }}."
      deny:
        conditions:
        - key: "{{ request.object.metadata.labels.animal }}"
          operator: NotIn
          value:  "{{ animals.data.animals }}"
```

Create `deploy.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: busybox
  labels:
    app: busybox
    color: red
    animal: cow
    food: pizza
    car: jeep
    env: qa
spec:
  replicas: 1
  selector:
    matchLabels:
      app: busybox
  template:
    metadata:
      labels:
        app: busybox
    spec:
      containers:
      - image: busybox:1.28
        name: busybox
        command: ["sleep", "9999"]
```

See that the Deployment fails now that Kyverno can read from multi-line YAML strings in a ConfigMap.

## CLI Support

A new feature which has been implemented for the webhook may often need to be available in the [Kyverno CLI](https://kyverno.io/docs/kyverno-cli/) simultaneously. Please ensure your tests and Proof Manifests include one for the `test` command allowing validation of the CLI functionality. If the provided functionality does not work in the CLI, a separate issue may need to be raised. 
