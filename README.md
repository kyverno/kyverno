# kube-policy
A Kubernetes native policy engine

## Motivation

## Examples

## How it works

# Installation

## Prerequisites

You need to have go and dep utils installed on your machine.
Ensure that GOPATH environment variable is set to desired location.
Code generation for CRD controller depends on kubernetes/hack, so before use code generation, execute:

`go get k8s.io/kubernetes/hack`

## You can `go get`

Due to the repository privacy, you should to add SSH key to your github user to clone repository using `go get` command.
Using `go get` you receive correct repository location ad $GOHOME/go/src which is needed to restore dependencies.
Configure SSH key due to this article: https://help.github.com/articles/adding-a-new-ssh-key-to-your-github-account/

After SSH key configured, you must tell git to use SSH. To do it use next command:

`git config --global url.git@github.com:.insteadOf https://github.com/`

After this is done, use next command to clone the repo:

`go get github.com/nirmata/kube-policy`

## Or `git clone`

If you don't want to use SSH, you just can clone repo with git, but ensure that repo will be inside this path: $GOPATH/src/.

`git clone https://github.com/nirmata/kube-policy.git $GOPATH/src/nirmata/kube-policy`

## Restore dependencies

Navigate to kube-policy project dir and execute:
`dep ensure`
This will install necessary dependencies described in README.md

# Contributing
