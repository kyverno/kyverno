# Kyverno [![Tweet](https://img.shields.io/twitter/url/http/shields.io.svg?style=social)](https://twitter.com/intent/tweet?text=Kubernetes%20Native%20Policy%20Management.%20No%20new%20language%20required%21&url=https://github.com/kyverno/kyverno/&hashtags=kubernetes,devops)

**Kubernetes Native Policy Management**

![build](https://github.com/kyverno/kyverno/workflows/build/badge.svg) 
![prereleaser](https://github.com/kyverno/kyverno/workflows/prereleaser/badge.svg) 
[![Go Report Card](https://goreportcard.com/badge/github.com/kyverno/kyverno)](https://goreportcard.com/report/github.com/kyverno/kyverno) 
![License: Apache-2.0](https://img.shields.io/github/license/kyverno/kyverno?color=blue)
[![GitHub Repo stars](https://img.shields.io/github/stars/kyverno/kyverno)](https://github.com/kyverno/kyverno/stargazers)


<a href="https://kyverno.io" rel="kyverno.io">![logo](img/Kyverno_Horizontal.png)</a>

<p class="callout info" style="font-size: 2000%;">
Kyverno is a policy engine designed for Kubernetes. It can validate, mutate, and generate configurations using admission controls and background scans. Kyverno policies are Kubernetes resources and do not require learning a new language. Kyverno is designed to work nicely with tools you already use like kubectl, kustomize, and Git.
</p>

## Documentation

Kyverno guides and reference documents are available at: <a href="https://kyverno.io/">kyverno.io</a>. 

Try the [quick start guide](https://kyverno.io/docs/introduction/#quick-start) to install Kyverno and create your first policy.

## Contributing

Checkout out the Kyverno <a href="https://kyverno.io/community">Community</a> page for ways to get involved and details on joining our next community meeting.

## Getting Help

- For feature requests and bugs, file an [issue](https://github.com/kyverno/kyverno/issues).
- For discussions or questions, join the **#kyverno** channel on the [Kubernetes Slack](https://kubernetes.slack.com/) or the [mailing list](https://groups.google.com/g/kyverno).


## pre-commit
pre-commit hook which runs kyverno docker image. This container can use github as a remote ref because it has been added to known hosts at build. For other git providers please raise an issue

## Example of .pre-commit-config.yaml that verifies that policies in the current repo 
```yaml
# See https://pre-commit.com for more information
# See https://pre-commit.com/hooks.html for more hooks
repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v2.4.0
    hooks:
    -   id: check-yaml
        args: [--allow-multiple-documents]
    -   id: check-added-large-files
-   repo: https://github.com/kyverno/kyverno
    rev: 1.3.4
    hooks:
    -   id: kyverno
        name: kyverno-validate
        args: [./]
        verbose: false
