## GenerateName Exclusion Conformance Test

### Overview
This conformance test verifies that admission requests using `metadata.generateName` prefixes cannot bypass policy validation by spoofing `excludeResourceRules.resourceNames`.

### Verification Steps
1. Deploy a `ValidatingPolicy` that enforces `env=prod` labels on `deployments`, while excluding `resourceNames: [trusted-deployment]`.
2. Attempt to create a deployment with `metadata.generateName: "trusted-deployment-"` and non-compliant labels (`env: testing`). The policy engine must NOT exclude this resource, subjecting it to validation and rejecting it (`$error != null: true`).
3. Create a legitimate deployment with exact `metadata.name: "trusted-deployment"` and non-compliant labels. The policy engine legitimately excludes this resource, permitting admission.
