<small>_[documentation](/README.md#documentation) / kyverno-cli_</small>

# Kyverno CLI

The Kyverno Command Line Interface (CLI) is designed to validate policies and test the behavior of applying policies to resources before adding the policy to a cluster. It can be used as a kubectl plugin and as a standalone CLI.

## Install the CLI

The Kyverno CLI binary is distributed with each release. You can install the CLI for your platform from the [releases](https://github.com/nirmata/kyverno/releases) site.

## Build the CLI

You can build the CLI binary locally, then move the binary into a directory in your PATH.

```bash
git clone https://github.com/nirmata/kyverno.git
cd github.com/nirmata/kyverno
make cli
mv ./cmd/cli/kubectl-kyverno/kyverno /usr/local/bin/kyverno
```

You can also use curl to install kyverno-cli

```bash
curl -L https://raw.githubusercontent.com/nirmata/kyverno/master/scripts/install-cli.sh | bash
```

## Install via AUR (archlinux)

You can install the kyverno cli via your favourite AUR helper (e.g. [yay](https://github.com/Jguer/yay))

```
yay -S kyverno-git
```

## Commands

#### Version

Prints the version of kyverno used by the CLI.

Example:

```
kyverno version
```

#### Validate

Validates a policy, can validate multiple policy resource description files or even an entire folder containing policy resource description
files. Currently supports files with resource description in YAML.

Example:

```
kyverno validate /path/to/policy1.yaml /path/to/policy2.yaml /path/to/folderFullOfPolicies
```

#### Apply

Applies policies on resources, and supports applying multiple policies on multiple resources in a single command.
Also supports applying the given policies to an entire cluster. The current kubectl context will be used to access the cluster.
Will return results to stdout.

Apply to a resource:

```bash
kyverno apply /path/to/policy.yaml --resource /path/to/resource.yaml
```

Apply to all matching resources in a cluster:

```bash
kyverno apply /path/to/policy.yaml --cluster > policy-results.txt
```

Apply multiple policies to multiple resources:

```bash
kyverno apply /path/to/policy1.yaml /path/to/folderFullOfPolicies --resource /path/to/resource1.yaml --resource /path/to/resource2.yaml --cluster
```

##### Exit Codes

The CLI exits with diffenent exit codes:

| Message                               | Exit Code |
| ------------------------------------- | --------- |
| executes successfully                 | 0         |
| one or more policy rules are violated | 1         |
| policy validation failed              | 2         |

<small>_Read Next >> [Sample Policies](/samples/README.md)_</small>
