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