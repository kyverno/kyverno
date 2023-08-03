# Security Policy
The Kyverno community has adopted this security disclosures and response policy to ensure we responsibly handle critical issues.

## Security bulletins
For information regarding the security of this project please join our [slack channel](https://slack.k8s.io/#kyverno).

## Reporting a Vulnerability
### When you should?
- You think you discovered a potential security vulnerability in Kyverno.
- You are unsure how a vulnerability affects Kyverno.
- You think you discovered a vulnerability in another project that Kyverno depends on. For projects with their own vulnerability reporting and disclosure process, please report it directly there.

### When you should not?
- You need help tuning Kyverno components for security - please discuss this is in the Kyverno [slack channel](https://slack.k8s.io/#kyverno).
- You need help applying security-related updates.
- Your issue is not security-related.

### Please use the below process to report a vulnerability to the project:
1. Email the **Kyverno security group at kyverno-security@googlegroups.com**
    * Emails should contain:
        * description of the problem
        * precise and detailed steps (include screenshots) that created the problem
        * the affected version(s)
        * any possible mitigations, if known
2. The project security team will send an initial response to the disclosure in 3-5 days. Once the vulnerability and fix are confirmed, the team will plan to release the fix in 7 to 28 days based on the severity and complexity.
3. You may be contacted by a project maintainer to further discuss the reported item. Please bear with us as we seek to understand the breadth and scope of the reported problem, recreate it, and confirm if there is a vulnerability present.

## Supported Versions
Kyverno versions follow [Semantic Versioning](https://semver.org/) terminology and are expressed as x.y.z:
- where x is the major version
- y is the minor version
- and z is the patch version

Security fixes, may be backported to the three most recent minor releases, depending on severity and feasibility. Patch releases are cut from those branches periodically, plus additional urgent releases, when required.

## Release Artifacts
The Kyverno container images are available [here](https://github.com/orgs/kyverno/packages).

## Signed-Releases
Signed releases attest to the provenance of the artifact.This check looks for the following filenames in the project's last five [release assets](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases): [*.minisig](https://github.com/jedisct1/minisign), *.asc (pgp), *.sig, *.sign, [*.intoto.jsonl](https://slsa.dev/).
To enable signed release, you need to follow the steps mentioned below for verifying Kyverno container images using Cosign. By verifying the signatures of the Kyverno container images, you can ensure our authenticity and reduce the risk of using malicious or tampered images. This helps to ensure that only signed and trusted releases of Kyverno are used.
Note: The check does not verify the signatures.

## Verifying Kyverno Container Images
Kyverno container images are signed using Cosign and the [keyless signing feature](https://docs.sigstore.dev/cosign/verify/). The signatures are stored in a separate repository from the container image they reference located at ```ghcr.io/kyverno/signatures```. To verify the container image using Cosign v1.x, follow the steps below.

1. Install [Cosign](https://github.com/sigstore/cosign#installation)
2. Configure the Kyverno signature repository:

```bash
  export COSIGN_REPOSITORY=ghcr.io/kyverno/signatures
```

3. Verify the image:
```bash
   COSIGN_EXPERIMENTAL=1 cosign verify ghcr.io/kyverno/kyverno:<release_tag> | jq
```
For Cosign v2.x, use the following command instead.

```bash
COSIGN_REPOSITORY=ghcr.io/kyverno/signatures cosign verify ghcr.io/kyverno/kyverno:<release_tag> \
  --certificate-identity-regexp="https://github.com/kyverno/kyverno/.github/workflows/release.yaml@refs/tags/*" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" | jq
```

If the container image was properly signed, the output provided here:
For more information, please refer to the documentation on security at [Verifying Kyverno Container Images.](https://kyverno.io/docs/security/#verifying-kyverno-container-images)

Note that the important fields to verify in the output are ```optional.Issuer``` and ```optional.Subject```. If Issuer and Subject do not match the values shown above, the image is not genuine.

All Kyverno images can be verified.

## Verifying Provenance 
Kyverno creates and attests to the provenance of its builds using the [SLSA standard](https://slsa.dev/provenance/v0.2) and meets the SLSA [Level 3](https://slsa.dev/spec/v0.1/levels) specification. The attested provenance may be verified using the ```cosign``` tool.

For v1.x of Cosign, use the following command.

```bash
COSIGN_EXPERIMENTAL=1 cosign verify-attestation \
  --type slsaprovenance ghcr.io/kyverno/kyverno:<release_tag> | jq .payload -r | base64 --decode | jq
```

For v2.x of Cosign, use the following command.

```bash
cosign verify-attestation --type slsaprovenance \
  --certificate-identity-regexp="https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@refs/tags/*" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/kyverno/kyverno:<release_tag> | jq .payload -r | base64 --decode | jq
```

For more information, please visit [Verifying Provenance.](https://kyverno.io/docs/security/#verifying-provenance)