<!--
Copyright 2024 The Kyverno Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# Kyverno [![Tweet](https://img.shields.io/twitter/url/http/shields.io.svg?style=social)](https://twitter.com/intent/tweet?text=Cloud%20Native%20Policy%20Management.%20No%20new%20language%20required%1&url=https://github.com/kyverno/kyverno/&hashtags=kubernetes,devops)

**Cloud Native Policy Management 🎉**

[![Go Report Card](https://goreportcard.com/badge/github.com/kyverno/kyverno)](https://goreportcard.com/report/github.com/kyverno/kyverno)
![License: Apache-2.0](https://img.shields.io/github/license/kyverno/kyverno?color=blue)
[![GitHub Repo stars](https://img.shields.io/github/stars/kyverno/kyverno)](https://github.com/kyverno/kyverno/stargazers)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/5327/badge)](https://bestpractices.coreinfrastructure.org/projects/5327)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/kyverno/kyverno/badge)](https://securityscorecards.dev/viewer/?uri=github.com/kyverno/kyverno)
[![SLSA 3](https://slsa.dev/images/gh-badge-level3.svg)](https://slsa.dev)
[![Artifact HUB](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/kyverno)](https://artifacthub.io/packages/search?repo=kyverno)
[![codecov](https://codecov.io/gh/kyverno/kyverno/branch/main/graph/badge.svg)](https://app.codecov.io/gh/kyverno/kyverno/branch/main)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fkyverno%2Fkyverno.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fkyverno%2Fkyverno?ref=badge_shield)


<a href="https://kyverno.io" rel="kyverno.io">![logo](img/Kyverno_Horizontal.png)</a>

<p class="callout info" style="font-size: 100%;">
Kyverno is a policy engine designed for cloud native platform engineering teams. It enables security, automation, compliance, and governance using policy-as-code. Kyverno can validate, mutate, generate, and cleanup configurations using Kubernetes admission controls, background scans, and source code respository scans. Kyverno policies can also be used to verify OCI images, for software supply chain security. Kyverno policies can be managed as Kubernetes resources and do not require learning a new language. Kyverno is designed to work nicely with tools you already use like kubectl, kustomize, and Git.
</p>

<a href="https://opensourcesecurityindex.io/" target="_blank" rel="noopener"> <img
        style="width: 282px; height: 56px"
        src="https://opensourcesecurityindex.io/badge.svg"
        alt="Open Source Security Index - Fastest Growing Open Source Security Projects"
        width="282"
        height="56"
    />
</a>

## 📙 Documentation

Kyverno installation and reference documents are available at [kyverno.io](https://kyverno.io).

👉 **[Quick Start](https://kyverno.io/docs/introduction/#quick-start)**

👉 **[Installation](https://kyverno.io/docs/installation/)**

👉 **[Sample Policies](https://kyverno.io/policies/)**

## 🎯 Popular Use Cases

Kyverno helps platform teams enforce best practices and security policies. Here are some common use cases:

1. **Security & Compliance**
   - Enforce pod security standards
   - Require specific security contexts
   - Validate image sources and signatures
   - Ensure resource limits and requests

2. **Operational Excellence**
   - Automatically add labels and annotations
   - Enforce naming conventions
   - Generate default network policies
   - Validate resource configurations

3. **Cost Optimization**
   - Enforce resource quotas
   - Require cost allocation labels
   - Clean up unused resources
   - Validate instance types

4. **Developer Guardrails**
   - Enforce ingress/egress rules
   - Require liveness/readiness probes
   - Validate container images
   - Auto-mount configuration

Each use case includes ready-to-use policies in our [policy library](https://kyverno.io/policies/).

## 🙋‍♂️ Getting Help

We are here to help!

👉 For feature requests and bugs, file an [issue](https://github.com/kyverno/kyverno/issues).

👉 For discussions or questions, join the [Kyverno Slack channel](https://slack.k8s.io/#kyverno).

👉 For community meeting access, see [mailing list](https://kyverno.io/community/#community-meetings).

👉 To get follow updates ⭐️ [star this repository](https://github.com/kyverno/kyverno/stargazers).

## ➕ Contributing

Thanks for your interest in contributing to Kyverno! Here are some steps to help get you started:

✔ Read and agree to the [Contribution Guidelines](/CONTRIBUTING.md).

✔ Browse through the [GitHub discussions](https://github.com/kyverno/kyverno/discussions).

✔ Read Kyverno design and development details on the [GitHub Wiki](https://github.com/kyverno/kyverno/wiki).

✔ Check out the [good first issues](https://github.com/kyverno/kyverno/labels/good%20first%20issue) list. Add a comment with `/assign` to request assignment of the issue.

✔ Check out the Kyverno [Community page](https://kyverno.io/community/) for other ways to get involved.

## Software Bill of Materials

All Kyverno images include a Software Bill of Materials (SBOM) in [CycloneDX](https://cyclonedx.org/) JSON format. SBOMs for Kyverno images are stored in a separate repository at `ghcr.io/kyverno/sbom`. More information on this is available at [Fetching the SBOM for Kyverno](https://kyverno.io/docs/security/#fetching-the-sbom-for-kyverno). 

## Contributors

Kyverno is built and maintained by our growing community of contributors!

<a href="https://github.com/kyverno/kyverno/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=kyverno/kyverno" />
</a>

Made with [contributors-img](https://contrib.rocks).

## License

Copyright 2025, the Kyverno project. All rights reserved. Kyverno is licensed under the [Apache License 2.0](LICENSE).

Kyverno is a [Cloud Native Computing Foundation (CNCF) Incubating project](https://www.cncf.io/projects/) and was contributed by [Nirmata](https://nirmata.com/?utm_source=github&utm_medium=repository).
