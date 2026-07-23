# Sigstore Trusted Roots

See blog post [cosign Verification of npm Provenance, GitHub Artifact Attestations, and Homebrew Provenance](https://blog.sigstore.dev/cosign-verify-bundles/)

To fetch the GitHub trusted roots run:

```sh
gh attestation trusted-root | jq '.|select(any(.certificateAuthorities[]; .uri=="fulcio.githubapp.com"))' > github-trusted-root.json
```
