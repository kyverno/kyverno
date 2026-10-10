# Kyverno + Alibaba Cloud ACR + Cosign Image Signature Verification

This guide walks through using Kyverno's `ImageValidatingPolicy` to perform
**Sigstore/Cosign** signature verification on private images hosted in Alibaba
Cloud Container Registry (ACR) Enterprise Edition, on an Alibaba Cloud ACK
(Container Service for Kubernetes) cluster.

The verification path exercises the full chain:

```text
Pod creation request
    -> Kyverno Admission Webhook
    -> ImageValidatingPolicy
    -> docker-credential-acr-helper (alibabacloud provider)
    -> RRSA obtains an STS token
    -> ACR OpenAPI (ListInstance + GetAuthorizationToken + PullRepository)
    -> pull image manifest / signature layer
    -> Cosign public key verification
    -> allow / deny Pod creation
```

> This guide documents a flow verified on an ACK cluster. It requires a Kyverno
> build that includes the `alibabacloud` registry credential helper (this branch).

---

## Contents

- [Prerequisites](#prerequisites)
- [Architecture](#architecture)
- [Step 1: Sign an ACR image with Cosign](#step-1-sign-an-acr-image-with-cosign)
- [Step 2: Alibaba Cloud RAM and RRSA setup](#step-2-alibaba-cloud-ram-and-rrsa-setup)
- [Step 3: Deploy / upgrade Kyverno](#step-3-deploy--upgrade-kyverno)
- [Step 4: Create the ImageValidatingPolicy](#step-4-create-the-imagevalidatingpolicy)
- [Step 5: Verification tests](#step-5-verification-tests)
- [Troubleshooting](#troubleshooting)
- [Security recommendations](#security-recommendations)

---

## Prerequisites

### 1. Alibaba Cloud side

| Resource | Description |
|----------|-------------|
| Alibaba Cloud account / RAM user | Used to create the Role, Policy, and operate ACR |
| ACR Enterprise Edition instance | With a namespace and repository created, and a test image pushed |
| ACK cluster | With **RRSA** enabled and the `ack-pod-identity-webhook` component installed |

### 2. Local tools

- `kubectl` configured for the target cluster
- `helm` 3.x
- `cosign` CLI (signing and verification)
- `docker` or `crane` (to push images to ACR)

### 3. Test image

Prepare an image already pushed to your ACR Enterprise Edition instance, e.g.:

```text
dahu-registry-vpc.cn-hangzhou.cr.aliyuncs.com/dahu/nginx:1.14.1-8.6
```

---

## Architecture

```text
┌─────────────────┐
│   ACK Cluster   │
│                 │
│  ┌───────────┐  │      ┌─────────────────────┐
│  │ Kyverno   │  │      │  ACR Enterprise     │
│  │ Admission │──┼─────▶│  ListInstance       │
│  │ Controller│  │      │  GetAuthorizationToken
│  └─────┬─────┘  │      │  PullRepository     │
│        │        │      └─────────────────────┘
│        │        │                ▲
│        │        │                │ RRSA STS Token
│        │        │      ┌─────────┴─────────┐
│        │        │      │  ACK RRSA/OIDC     │
│        │        │      └────────────────────┘
│        ▼        │
│  ImageValidating│
│  Policy (cosign)│
│                 │
└─────────────────┘
```

Kyverno enables `docker-credential-acr-helper` via
`registryCredentialHelpers: alibabacloud`. The helper:

1. Obtains an STS token from the RRSA environment variables;
2. Calls `cr:ListInstance` to map the registry domain to an instance ID;
3. Calls `cr:GetAuthorizationToken` to obtain a temporary username/password;
4. Uses those temporary credentials to pull the image and signature layers
   from ACR;
5. Verifies the signature with the Cosign public key configured in the
   `ImageValidatingPolicy`.

> Note: the temporary credentials must also have `cr:PullRepository`
> permission, otherwise pulling the manifest returns 401 Unauthorized even
> after a token is obtained.

---

## Step 1: Sign an ACR image with Cosign

### 1.1 Generate a Cosign key pair

```bash
cd <working-dir>

# generates cosign.key / cosign.pub (keep the private key safe)
cosign generate-key-pair
```

### 1.2 Log in to ACR and sign

```bash
# log in with your Alibaba Cloud account or a RAM user
docker login --username=<your-username> dahu-registry-vpc.cn-hangzhou.cr.aliyuncs.com

# sign the image
export IMAGE="dahu-registry-vpc.cn-hangzhou.cr.aliyuncs.com/dahu/nginx:1.14.1-8.6"
cosign sign --key cosign.key "${IMAGE}"

# verify the signature locally
cosign verify --key cosign.pub "${IMAGE}"
```

> The `sign-image.sh` script in this directory automates these steps; edit its
> `IMAGE` variable first.

After signing, the image gains a `.sig` signature layer in the ACR repository.

---

## Step 2: Alibaba Cloud RAM and RRSA setup

### 2.1 Create the RAM Role

Create a role `kyverno-acr-role` with the following trust policy:

```json
{
  "Version": "1",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "sts:AssumeRole",
      "Principal": {
        "Federated": [
          "acs:ram::<account-id>:oidc-provider/ack-rrsa-<cluster-id>"
        ]
      },
      "Condition": {
        "StringEquals": {
          "oidc:iss": "https://oidc-ack-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com/<cluster-id>",
          "oidc:aud": "sts.aliyuncs.com",
          "oidc:sub": [
            "system:serviceaccount:kyverno:kyverno-admission-controller",
            "system:serviceaccount:kyverno:kyverno-background-controller",
            "system:serviceaccount:kyverno:kyverno-cleanup-controller",
            "system:serviceaccount:kyverno:kyverno-reports-controller"
          ]
        }
      }
    }
  ]
}
```

> Replace `<account-id>` and `<cluster-id>` with your values. See
> `rrsa-trust-policy.json` for a template.

### 2.2 Create and attach the RAM Policy

Create a custom policy and attach it to `kyverno-acr-role`. Minimal
permissions:

```json
{
  "Version": "1",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "cr:GetAuthorizationToken",
        "cr:ListInstance",
        "cr:PullRepository"
      ],
      "Resource": "*"
    }
  ]
}
```

See `ram-policy.json`.

What each action is for:

| Action | Purpose |
|--------|---------|
| `cr:GetAuthorizationToken` | Obtain temporary ACR login credentials (username/password) |
| `cr:ListInstance` | Map the registry domain to an ACR instance ID |
| `cr:PullRepository` | Actually pull the image manifest/signature with the temporary user |

To minimize `Resource`, replace `"*"` with:

```json
"Resource": [
  "acs:cr:cn-hangzhou:*:repository/<instance-id>/<namespace>/<repo-name>"
]
```

### 2.3 Enable RRSA injection for the Kyverno namespace

```bash
kubectl label namespace kyverno pod-identity.alibabacloud.com/injection=on
```

---

## Step 3: Deploy / upgrade Kyverno

### 3.1 RRSA (recommended)

Use `helm-values-extra.yaml`:

```yaml
admissionController:
  container:
    extraArgs:
      registryCredentialHelpers: alibabacloud
  rbac:
    serviceAccount:
      annotations:
        pod-identity.alibabacloud.com/role-name: kyverno-acr-role

backgroundController:
  container:
    extraArgs:
      registryCredentialHelpers: alibabacloud
  rbac:
    serviceAccount:
      annotations:
        pod-identity.alibabacloud.com/role-name: kyverno-acr-role

cleanupController:
  container:
    extraArgs:
      registryCredentialHelpers: alibabacloud
  rbac:
    serviceAccount:
      annotations:
        pod-identity.alibabacloud.com/role-name: kyverno-acr-role

reportsController:
  container:
    extraArgs:
      registryCredentialHelpers: alibabacloud
  rbac:
    serviceAccount:
      annotations:
        pod-identity.alibabacloud.com/role-name: kyverno-acr-role
```

Upgrade Kyverno:

```bash
helm upgrade kyverno /path/to/kyverno/chart \
  -n kyverno \
  -f /path/to/kyverno-pre-values.yaml \
  -f test/aliyun/helm-values-extra.yaml
```

After the upgrade, check the four controllers:

```bash
kubectl get pods -n kyverno
kubectl get sa -n kyverno kyverno-admission-controller -o yaml | grep pod-identity
```

Confirm the service account carries the annotation:

```yaml
pod-identity.alibabacloud.com/role-name: kyverno-acr-role
```

and the pods have been injected with:

- `ALIBABA_CLOUD_ROLE_ARN`
- `ALIBABA_CLOUD_OIDC_PROVIDER_ARN`
- `ALIBABA_CLOUD_ROLE_SESSION_NAME`
- the projected token under `/var/run/secrets/pod-identity-token/`

---

## Step 4: Create the ImageValidatingPolicy

Paste the content of `cosign.pub` into `verify-acr-image-policy.yaml`:

```yaml
apiVersion: policies.kyverno.io/v1beta1
kind: ImageValidatingPolicy
metadata:
  name: verify-acr-image-cosign
spec:
  failurePolicy: Ignore
  validationActions:
    - Deny
  webhookConfiguration:
    timeoutSeconds: 20
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["pods"]
  matchConditions:
    - name: match-test-namespace
      expression: "request.namespace == 'kyverno-cap-test'"
  matchImageReferences:
    - glob: "dahu-registry-vpc.cn-hangzhou.cr.aliyuncs.com/dahu/nginx*"
  credentials:
    providers:
      - alibabacloud
  validationConfigurations:
    mutateDigest: false
    verifyDigest: false
    required: true
  attestors:
    - name: cosign
      cosign:
        key:
          data: |-
            -----BEGIN PUBLIC KEY-----
            <contents of cosign.pub>
            -----END PUBLIC KEY-----
  validations:
    - expression: >-
        images.containers.map(image, verifyImageSignatures(image, [attestors.cosign])).all(e, e > 0)
      message: "ACR image signature verification failed"
```

Key fields:

| Field | Description |
|-------|-------------|
| `failurePolicy: Ignore` | Don't block Pod creation if the webhook itself fails (`Ignore` is recommended during evaluation so a Kyverno outage doesn't block the cluster) |
| `validationActions: [Deny]` | Deny the request when verification fails |
| `matchImageReferences` | Restrict matching to the given ACR images (glob syntax) |
| `credentials.providers: [alibabacloud]` | Enable the ACR credential helper |
| `validationConfigurations.required: true` | All matched images must verify |
| `attestors.cosign.key.data` | Public key used to verify signatures |
| `validations` | CEL expression running `verifyImageSignatures` over all container images |

Apply the policy:

```bash
kubectl apply -f test/aliyun/verify-acr-image-policy.yaml
```

Check its status:

```bash
kubectl get imagevalidatingpolicy verify-acr-image-cosign -o yaml
```

---

## Step 5: Verification tests

### 5.1 Signed image (expected to pass)

```bash
kubectl apply -f test/aliyun/signed-pod.yaml
```

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: signed-nginx
  namespace: kyverno-cap-test
spec:
  containers:
    - name: nginx
      image: dahu-registry-vpc.cn-hangzhou.cr.aliyuncs.com/dahu/nginx:1.14.1-8.6
```

Expected result:

```bash
kubectl get pod -n kyverno-cap-test signed-nginx
# NAME           READY   STATUS    RESTARTS   AGE
# signed-nginx   1/1     Running   0          1m
```

### 5.2 Unsigned image (expected to be denied)

```bash
kubectl apply -f test/aliyun/unsigned-pod.yaml
```

Expected result:

```text
Error from server: error when creating "unsigned-pod.yaml": admission webhook "validate.kyverno.io-*" denied the request:
ACR image signature verification failed
```

---

## Troubleshooting

### 1. Pod creation rejected with 401 Unauthorized

**Symptom**: Kyverno logs show `UNAUTHORIZED` or `401`.

**Investigation**:

```bash
# 1. Confirm the RRSA env vars are injected into the admission controller pod
kubectl exec -n kyverno deploy/kyverno-admission-controller -- env | grep ALIBABA

# 2. Run a debug pod using the kyverno-admission-controller service account
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: rrsa-debug
  namespace: kyverno
spec:
  serviceAccountName: kyverno-admission-controller
  containers:
    - name: debug
      image: registry.cn-hangzhou.aliyuncs.com/acs/alpine:3.18
      command: ["sleep", "3600"]
EOF

# 3. Test the ACR credential chain from inside the debug pod
kubectl exec -n kyverno rrsa-debug -- <acr-helper-binary>
```

**Common root causes**:

| Error | Cause | Fix |
|-------|-------|-----|
| `NoPermission` / `ListInstance denied` | RAM policy is missing `cr:ListInstance` | Add `cr:ListInstance` to the RAM policy |
| Token obtained but pull returns 401 | RAM policy is missing `cr:PullRepository` | Add `cr:PullRepository` to the RAM policy |
| `AssumeRole denied` | `oidc:sub` in the trust policy doesn't match the service account | Check the service account paths in the RRSA trust policy |
| No RRSA env vars | Namespace not labeled or webhook not installed | `kubectl label ns kyverno pod-identity.alibabacloud.com/injection=on` |

### 2. The ImageValidatingPolicy is not taking effect

- Confirm the pod's namespace satisfies the `matchConditions`;
- Confirm the image reference matches the `matchImageReferences` glob;
- Confirm the Kyverno pods restarted and loaded the new policy;
- Check whether the policy status reports `Ready=True`.

### 3. Cosign verification failures

- Confirm the private key used for signing pairs with the public key in the policy;
- Confirm the image tag wasn't overwritten or re-pushed (which invalidates the signature);
- Verify locally first with `cosign verify --key cosign.pub <image>`.

---

## Security recommendations

1. **Use RRSA in production** for temporary, automatically rotated credentials.
2. **Minimize the RAM policy**:
   - Tighten `Resource` from `*` to specific instance / namespace / repository;
   - Do not grant write permissions such as `cr:PushRepository` to the Kyverno role.
3. **Protect `cosign.key`**: never commit the private key; manage it with KMS or
   HashiCorp Vault.
4. **Webhook `failurePolicy`**: keep `Ignore` during evaluation so a Kyverno
   outage doesn't block the whole cluster; consider `Fail` only for
   security-critical environments.
5. **Rotate the Cosign key pair regularly**, and update the public key in the
   `ImageValidatingPolicy` accordingly.

---

## Files in this directory

| File | Purpose |
|------|---------|
| `sign-image.sh` | Generate a Cosign key pair and sign the image |
| `ram-policy.json` | RAM role permission policy |
| `rrsa-trust-policy.json` | RRSA role trust policy template |
| `verify-acr-image-policy.yaml` | Example ImageValidatingPolicy |
| `helm-values-extra.yaml` | Helm values for the RRSA setup |
| `signed-pod.yaml` | Test pod using a signed image |
| `unsigned-pod.yaml` | Test pod using an unsigned image |
