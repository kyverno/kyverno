# kube-policy
A Kubernetes native policy engine

## Motivation

## Examples

## How it works

# Build

## Prerequisites

You need to have go and dep utils installed on your machine.
Ensure that GOPATH environment variable is set to desired location.
Code generation for CRD controller depends on kubernetes/hack, so before use code generation, execute:

`go get k8s.io/kubernetes/hack`

We are using [dep](https://github.com/golang/dep)

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

## Compiling

We are using code generator for custom resources objects from here: https://github.com/kubernetes/code-generator

Generate the additional controller code before compiling the project:

`scripts/update-codegen.sh`

Then you can build the controller:

`go build .`

# Installation

There are 2 possible ways to install and use the controller: for **development** and for **production**

## For development

_At the time of this writing, only this installation method worked_

1. Open your `~/.kube/config` file and copy the value of `certificate-authority-data` to the clipboard
2. Open `crd/MutatingWebhookConfiguration_local.yaml` and replace `${CA_BUNDLE}` with the contents of clipboard
3. Open `~/.kube/config` again and copy the ip of the `server` value, for example `192.168.10.117`
4. Run `scripts/deploy-controller.sh --service=localhost --serverIp=<server_IP>` where `<server_IP>` is a server from clipboard. This scripts will generate TLS certificate for webhook server and register this webhook in the cluster. Also it registers CustomResource `Policy`.
5. Start controller: `sudo kube-policy --cert=certs/server.crt --key=certs/server-key.pem --kubeconfig=~/.kube/config`

## For production

_To be implemented_