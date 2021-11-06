# Contributing to Kyverno

We welcome all contributions, suggestions, and feedback, so please do not hesitate to reach out!

## Ways you can contribute

- [Report Issues](#report-issues)
- [Submit Pull Requests](#submit-pull-requests)
- [Fix or Improve Documentation](#fix-or-improve-documentation)
- [Join Our Community Meetings](#join-our-community-meetings)

### Report issues

Issues to Kyverno help improve the project in multiple ways including the following:

- Report potential bugs
- Request a feature
- Request a sample policy

### Submit Pull Requests

[Pull requests](https://docs.github.com/en/github/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/about-pull-requests) (PRs) allow you to contribute back the changes you've made on your side enabling others in the community to benefit from your hard work. They are the main source by which all changes are made to this project and are a standard piece of GitHub operational flows. Before you contribute, please take a moment to review and agree to abide by our community [Code of Conduct](/CODE_OF_CONDUCT.md).

New contributors may easily view all [open issues labeled as good first issues](https://github.com/kyverno/kyverno/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) allowing you to get started in an approachable manner.

Once you wish to get started contributing to the code base, please refer to our [running in development mode](https://github.com/kyverno/kyverno/wiki/Running-in-development-mode) for local setup guide.

In the process of submitting your PRs, please read and abide by the template provided to ensure the maintainers are able to understand your changes and quickly come up to speed. There are some important pieces that are required outside of the code itself. Some of these are up to you, others are up to the maintainers.

1. Provide Proof Manifests allowing the maintainers and other contributors to verify your changes without requiring they understand the nuances of all your code.
2. For new or changed functionality, this typically requires documentation and so raise a corresponding issue (or, better yet, raise a separate PR) on the [documentation repository](https://github.com/kyverno/website).
3. Test your change with the [Kyverno CLI](https://kyverno.io/docs/kyverno-cli/) and provide a test manifest in the proper format. If your feature/fix does not work with the CLI, a separate issue requesting CLI support must be made.
4. Indicate which release this PR is triaged for (maintainers). This step is important especially for the documentation maintainers in order to understand when and where the necessary changes should be made.

#### Getting started with your PR

Head over to the project repository on GitHub and click the **"Fork"** button. With the forked copy, you can try new ideas and implement changes to the project.

**Clone the repository to your device:**

Get the link of your forked repository, paste it in your device terminal and clone it using the command.

```sh
git clone https://hostname/YOUR-USERNAME/YOUR-REPOSITORY
```

**Create a branch:**

Create a new brach and navigate to the branch using this command.

```sh
git checkout -b <new-branch>
```

Great, it's time to start hacking! You can now go ahead to make all the changes you want.

**Stage, Commit, and Push changes:**

Now that we have implemented the required changes, use the command below to stage the changes and commit them.

```sh
git add .
```

```sh
git commit -s -m "Commit message"
```

The `-s` signifies that you have signed off the the commit.

Go ahead and push your changes to GitHub using this command.

```sh
git push 
```

### Fix or Improve Documentation

The [Kyverno website](https://kyverno.io), like the main Kyverno codebase, is stored in its own [git repo](https://github.com/kyverno/website). To get started with contributions to the documentation, [follow the guide](https://github.com/kyverno/website#contributing) on that repository.

### Engage with us

The website has the most updated information on [how to engage with the Kyverno community](https://kyverno.io/community/) including its maintainers and contributors.

## Developer Certificate of Origin (DCO) Sign off

For contributors to certify that they wrote or otherwise have the right to submit the code they are contributing to the project, we are requiring everyone to acknowledge this by signing their work which indicates you agree to the DCO found [here](https://developercertificate.org/).

To sign your work, just add a line like this at the end of your commit message:

```sh
Signed-off-by: Random J Developer <random@developer.example.org>
```

This can easily be done with the `-s` command line option to append this automatically to your commit message.

```sh
git commit -s -m 'This is my commit message'
```
