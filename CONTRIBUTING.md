# Contributing to Kyverno

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->


- [Introduction](#introduction)
- [Contributing Code](#contributing-code)
- [Disclosing vulnerabilities](#disclosing-vulnerabilities)
- [Code Style](#code-style)
- [Pull request procedure](#pull-request-procedure)
- [Communication](#communication)
- [Conduct](./CODE_OF_CONDUCT.md)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Introduction

Please note: We take Kyverno security and our users' trust very
seriously. If you believe you have found a security issue in Kyverno,
please responsibly disclose by contacting us at support@nirmata.com.

First: if you're unsure or afraid of anything, just ask or submit the issue or
pull request anyways. You won't be yelled at for giving it your best effort. The
worst that can happen is that you'll be politely asked to change something. We
appreciate any sort of contributions, and don't want a wall of rules to get in
the way of that.

That said, if you want to ensure that a pull request is likely to be merged,
talk to us! You can find out our thoughts and ensure that your contribution
won't clash or be obviated by Kyverno normal direction. A great way to
do this is via the [Kyverno Community](https://app.slack.com/client/T09NY5SBT/CLGR9BJU9)

## Contributing Code

Unless you are fixing a known bug, we **strongly** recommend discussing it with
the core team via a GitHub issue or [in our chat](https://app.slack.com/client/T09NY5SBT/CLGR9BJU9)
before getting started to ensure your work is consistent with Kyverno
roadmap and architecture.

All contributions are made via pull request. Note that **all patches from all
contributors get reviewed**. After a pull request is made other contributors
will offer feedback, and if the patch passes review a maintainer will accept it
with a comment. When pull requests fail testing, authors are expected to update
their pull requests to address the failures until the tests pass and the pull
request merges successfully.

At least one review from a maintainer is required for all patches (even patches
from maintainers).

Reviewers should leave a "LGTM" comment once they are satisfied with the patch.
If the patch was submitted by a maintainer with write access, the pull request
should be merged by the submitter after review.

## Disclosing vulnerabilities

Please disclose vulnerabilities exclusively to [support@nirmata.com](mailto:support@nirmata.com). Do
not use GitHub issues.

## Code Style

We follow the community provided standard [code structure](https://github.com/golang-standards/project-layout). Please follow these guidelines when formatting source code:

- Go code should match the output of `gofmt -s`

## Pull request procedure

To make a pull request, you will need a GitHub account; if you are unclear on
this process, see GitHub's documentation on
[forking](https://help.github.com/articles/fork-a-repo) and
[pull requests](https://help.github.com/articles/using-pull-requests). Pull
requests should be targeted at the `master` branch. Before creating a pull
request, go through this checklist:

1. Create a feature branch off of `master` so that changes do not get mixed up.
1. [Rebase](http://git-scm.com/book/en/Git-Branching-Rebasing) your local
   changes against the `master` branch.
1. Run the full project test suite with the `go test ./...` (or equivalent)
   command and confirm that it passes.
1. Run `gofmt -s` (if the project is written in Go).
1. Ensure that each commit has a subsystem prefix (ex: `controller:`).

Pull requests will be treated as "review requests," and maintainers will give
feedback on the style and substance of the patch.

Normally, all pull requests must include tests that test your change.
Occasionally, a change will be very difficult to test for. In those cases,
please include a note in your commit message explaining why.

## Communication

We use [Slack](https://app.slack.com/client/T09NY5SBT/CLGR9BJU9). You are welcome to drop in and ask
questions, discuss bugs, etc.

