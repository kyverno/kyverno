# Getting Started with Kyverno: An Orientation Guide

## What Problem This Guide Solves

Kyverno has comprehensive documentation covering everything from installation to advanced policy techniques. However, for many new users, the challenge isn't a lack of information—it’s knowing which path to take first. With multiple ways to write and apply policies, it can be difficult to see how they all fit together.

This guide is designed to help you find your footing. It focuses on the "why" and "where" rather than the "how," helping you navigate the existing documentation more effectively.

## Kyverno Policy Approaches (Conceptual)

Kyverno offers two main ways to think about and write policies. They are not competing standards; instead, they coexist to give you the right tool for the right task.

*   **Traditional Kyverno Policies:** These use a declarative YAML-based pattern. They are designed to be easy to read and write for anyone familiar with Kubernetes manifests. They excel at standard validation, mutation, and resource generation using simple match/exclude logic.
*   **CEL-Based Policies:** These leverage Common Expression Language (CEL), a fast and portable expression language. CEL allows for more complex logic and dynamic conditions within your policies.

You do not need to choose one over the other. Most environments use both: traditional policies for common guardrails and CEL-based policies for more nuanced or logic-heavy requirements.

## How to Choose Where to Start

If you are feeling unsure, a common and gentle way to begin is with **Validation**.

Validation policies simply ask: "Does this resource meet our requirements?" They are a great starting point because they don't change anything in your cluster. You can even run them in an audit-only mode to see what *would* have happened before you start enforcing rules.

As you become more comfortable, you might find yourself moving toward **Mutation** (automatically fixing resources) or **Generation** (creating new resources based on triggers).

The best place to start is usually with the problem you are trying to solve right now, rather than trying to learn every policy type at once.

## Where to Go Next

Once you have a specific goal in mind, these sections of the official documentation will help you move forward:

*   **For a hands-on introduction:** The [Quick Start Guide](https://kyverno.io/docs/introduction/quick-start/) provides a clear path through the most common use cases.
*   **To see what’s possible:** Browse the [Policy Library](https://kyverno.io/policies/) for hundreds of community-contributed examples that you can adapt for your own needs.
*   **To understand the different resources:** Read about [Policy Types](https://kyverno.io/docs/policy-types/) to understand the difference between a `ClusterPolicy`, a `Policy`, and the newer specialized sub-types.
*   **To refine your workflow:** Once you start writing, the [Tips & Tricks](https://kyverno.io/docs/writing-policies/tips/) section offers practical advice on testing and debugging your policies before they reach your cluster.

Kyverno is a flexible tool, and its documentation is deep. Take your time, start small, and use these resources as you need them.

