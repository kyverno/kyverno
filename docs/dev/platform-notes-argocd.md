---
title: Platform Notes (Argo CD)
excerpt: Special considerations for deploying Kyverno with Argo CD.
---



Argo CD v2.10 introduced support for `ServerSideDiff`, leveraging Kubernetes' Server Side Apply feature to resolve OutOfSync issues. This strategy ensures comparisons are handled on the server side, respecting fields like `skipBackgroundRequests` that Kubernetes sets by default, and fields set by mutating admission controllers like Kyverno, thereby preventing unnecessary `OutOfSync` errors caused by local manifest discrepancies.

> **Argo CD v3 note:** Starting with Argo CD v3 (released May 2025), the default resource-tracking method changed from label-based tracking (`app.kubernetes.io/instance`) to **annotation-based tracking** ([see upgrade notes](https://argo-cd.readthedocs.io/en/stable/operator-manual/upgrading/2.14-3.0/#use-annotation-based-tracking-by-default)). For Argo CD v3+ deployments using default settings the label-tracking conflict described in the *Notes for Argo CD Users* section below no longer applies unless you have explicitly configured Argo CD to use label tracking.

## Configuration Best Practices

1. **Server-Side Configuration**
   - Enable `ServerSideDiff` in one of two ways:
     - Per Application: Add the `argocd.argoproj.io/compare-options` annotation
     - Globally: Configure it in the `argocd-cmd-params-cm` ConfigMap

   ```yaml
   apiVersion: argoproj.io/v1alpha1
   kind: Application
   metadata:
     annotations:
       argocd.argoproj.io/compare-options: ServerSideDiff=true,IncludeMutationWebhook=true
       #...
   ```

2. **RBAC and CRD Management**
   - [Enable ServerSideApply](https://argo-cd.readthedocs.io/en/stable/user-guide/sync-options/#server-side-apply) in the `syncOptions` to handle metadata properly
   - Configure Argo CD to [ignore differences in aggregated ClusterRoles](https://argo-cd.readthedocs.io/en/stable/user-guide/diffing/#ignoring-rbac-changes-made-by-aggregateroles)
   - Ensure proper RBAC permissions for Argo CD to manage Kyverno CRDs

3. **Sync Options Configuration**
   - Avoid using `Replace=true` as it may cause issues with existing resources
   - Use `ServerSideApply=true` for smooth resource updates
   - Enable `CreateNamespace=true` if deploying to a new namespace

4. **Config Preservation**
   - By default, `config.preserve=true` is set in the Helm chart. This is useful for Helm-based install, upgrade, and uninstall scenarios.
   - This setting adds `helm.sh/resource-policy: "keep"` to the Kyverno ConfigMap, which prevents Helm and Argo CD from removing the ConfigMap on deletion. This can cause Argo CD to show the application as out-of-sync in App of Apps patterns.
   - It may also prevent Argo CD from cleaning up the Kyverno application when the parent application is deleted, since the ConfigMap with the keep policy remains in the cluster.
   - **Recommendation:** Set `config.preserve=false` when deploying Kyverno via Argo CD to ensure proper resource cleanup and sync status.

5. **Webhook Labels (`config.webhookLabels`)**
   - Kyverno can add labels to its webhook configurations via the `config.webhookLabels` Helm value (nested under `config`, not at the top level).
   - This is helpful when viewing the Kyverno application in Argo CD — it links the webhook resources to the Kyverno application so they are visible in the Argo CD UI.
   - For Argo CD v2 (label-based tracking), set the `app.kubernetes.io/instance: kyverno` label so Argo CD can associate the webhooks with the app.
   - For Argo CD v3 (annotation-based tracking, the new default), you generally do not need to set these labels, as Argo CD uses the `argocd.argoproj.io/tracking-id` annotation instead. However, if you have configured Argo CD v3 to use hybrid annotation+label tracking, setting `app.kubernetes.io/managed-by: argocd` and `argocd.argoproj.io/instance: kyverno` can be used.

## Complete Application Example

The following example includes all recommended settings for deploying Kyverno via Argo CD:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kyverno
  namespace: argocd
  annotations:
    argocd.argoproj.io/compare-options: ServerSideDiff=true,IncludeMutationWebhook=true
spec:
  destination:
    namespace: kyverno
    server: https://kubernetes.default.svc
  project: default
  source:
    chart: kyverno
    repoURL: https://kyverno.github.io/kyverno
    targetRevision: <my.target.version>
    helm:
      values: |
        # Disable config preservation so Argo CD can properly clean up on delete
        # and avoid out-of-sync status in App of Apps patterns.
        config:
          preserve: false
          # Add labels to Kyverno's webhook configurations so they are visible
          # as part of the Kyverno application in Argo CD.
          # For Argo CD v3+ (annotation-based tracking, the default since v3):
          webhookLabels:
            app.kubernetes.io/managed-by: argocd
            # Optionally also add this label to associate webhooks with the
            # 'kyverno' application in both Argo CD v2 and v3:
            # argocd.argoproj.io/instance: kyverno
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
```

> **Note on `webhookLabels`:** This Helm value is nested under `config` (i.e., `config.webhookLabels`), not at the top level. Using a top-level `webhookLabels:` key in `helm.values` will have no effect.

## Troubleshooting Guide

1. **CRD Check Failures**
   - **Symptom**: Deployment fails during CRD validation
   - **Common Causes**:
     - Insufficient RBAC permissions
     - CRDs not properly registered
   - **Resolution**:
     - Verify RBAC permissions for Argo CD service account
     - Ensure CRDs are installed before policies
     - Check Argo CD logs for specific permission errors

2. **Sync Failures**
   - **Symptom**: Resources show as OutOfSync
   - **Common Causes**:
     - Missing ServerSideDiff configuration
     - Aggregated ClusterRole differences
   - **Resolution**:
     - Enable ServerSideDiff as shown above
     - Configure resource exclusions for aggregated roles
     - Check resource health status in Argo CD UI

3. **Resource Management Issues**
   - **Symptom**: Resources not properly created or updated
   - **Common Causes**:
     - Incorrect sync options
     - Resource ownership conflicts
   - **Resolution**:
     - Use ServerSideApply instead of Replace
     - Configure resource tracking method
     - Verify resource ownership labels

4. **Performance and Scaling**
   - **Symptom**: Slow syncs or resource processing
   - **Common Causes**:
     - Large number of resources
     - Resource intensive operations
   - **Resolution**:
     - Use selective sync for large deployments
     - Configure appropriate resource limits
     - Enable background processing where applicable

For considerations when using Argo CD along with Kyverno mutate policies, see [Kyverno mutate policy guidance for Argo CD](/docs/policy-types/cluster-policy/mutate#argocd).

## Resource Tracking and Ownership

## Notes for Argo CD Users

**Argo CD v3+ (default: annotation-based tracking)**

Starting with Argo CD v3, the default tracking method is **annotation-based tracking**. Argo CD uses the `argocd.argoproj.io/tracking-id` annotation to track resources, so the `app.kubernetes.io/instance` label conflict described below no longer affects default Argo CD v3 deployments.

If you are running Argo CD v3 with default settings, you do **not** need to change the tracking method; the label conflict does not apply.

**Argo CD v2 (default: label-based tracking)**

In Argo CD v2, Argo CD automatically sets the `app.kubernetes.io/instance` label and uses it to determine which resources form the app. The Kyverno Helm chart also sets this label for the same purposes. To resolve this conflict:

1. Configure Argo CD to use annotation-based tracking instead of label-based tracking, as described in the [Argo CD documentation](https://argo-cd.readthedocs.io/en/latest/user-guide/resource_tracking/#additional-tracking-methods-via-an-annotation). Add the following to `argocd-cm`:

   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: argocd-cm
     namespace: argocd
   data:
     application.resourceTrackingMethod: annotation
   ```

2. Alternatively, add the `argocd.argoproj.io/tracking-id` annotation to your Application manifest so Argo CD uses annotation tracking for that specific application:

   ```yaml
   apiVersion: argoproj.io/v1alpha1
   kind: Application
   metadata:
     name: kyverno
     namespace: argocd
     annotations:
       # Switch this application to annotation-based tracking
       # to avoid conflict with Kyverno's app.kubernetes.io/instance label
       argocd.argoproj.io/tracking-id: ""
   ```

Argo CD users may also have Kyverno add labels to webhooks via the `config.webhookLabels` key in the Kyverno Helm chart (which sets `webhookLabels` in the [Kyverno ConfigMap](/docs/installation/customization#configmap-keys)). This is helpful when viewing the Kyverno application in Argo CD. The label `app.kubernetes.io/managed-by: argocd` indicates these webhook resources are managed by Argo CD.
