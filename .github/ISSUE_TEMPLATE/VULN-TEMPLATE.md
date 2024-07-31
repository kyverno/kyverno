---
name: Vulnerability Report
about: Report detected vulnerabilities in the kyverno image
title: Vulnerabilities detected in {{ env.ARTIFACT_NAME }}
labels: security
---

High or critical vulnerabilities detected in {{ env.ARTIFACT_NAME }}. Scan results are below:

{{ env.RESULTS }}