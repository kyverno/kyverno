## v1.6.0-rc1
### Note
- Helm charts are changed to enforce PodDisruptionBudget for multi-replica clusters and PDB is removed from install manifests.

## v1.5.0-rc1
### Note
- The Helm CRDs was switched back to kyverno chart. To upgrade using Helm, please refer to https://github.com/kyverno/website/pull/304.
- With the change of dynamic webhooks, the readiness of the policy is reflected by `.status.ready`, When ready, it means the policy is ready to serve the admission requests.

### Deprecation
- To add a consistent style in flag names the following flags have been deprecated `webhooktimeout`, `gen-workers`,`disable-metrics`, `background-scan`, `auto-update-webhooks`, `profile-port`, `metrics-port` these will be removed in 1.6.0. The new flags are `webhookTimeout`, `genWorkers`, `disableMetrics`, `backgroundScan`, `autoUpdateWebhooks`,`profilePort`, `metricsPort` (#1991).

### Features
- Feature/foreach validate #2443
- Feature/foreach mutate #2493
- Feature/cosign attest #2487
- Make webhooks configurable #1981
- FailurePolicy `Ignore` vs `enforcing` policies #893
- Make failurePolicy configurable per Kyverno policy #1995
- Add feature gate flag "auto-update-webhooks" #2321
- Extend the "kyverno test" command to handle mutate policies #1821

### Enhancements
- Integrate Github Action #2349
- Use a custom repository with verifyImages #2294
- Add pod anti-affinity to Kyverno #1966
- Rename 'policies.kyverno.io/patches' to reflect actual functionality #1528
- Add global variables to CLI #1472
- Allow configuration of test image through chart values #2410
- Switch Helm CRDs back to kyverno chart and moving Policies to dedicated chart #2355
- Updating Contribution Markdown #2450
- Validate GVK in `match`/`exclude` block #2389
- Add `PodDisruptionBudget` in Kustomize & Helm #1979
- Upgrade Kyverno managed webhook configurations to v1 #2424
- Allow background scanning if only request.operation is used in preconditions #1883
- Add security vulnerability scan for the kyverno images #1557
- Run vulnerability scan during Kyverno builds #2432
- Sign Kyverno images and generate SBOM #2175
- Make flag name styles consistent #1991
- Improve init container to use DeleteCollection to remove policy reports #2477
- Leader election for initContianer #1965
- Sample policies should have related CLI apply/test #1994


### Bug Fixes
- Autogen-controllers does not work with "any" rules #2337
- Use `patchesJson6902` where path contains a non-zero index number causes validation failure #2100
- CLI apply command - not filtering the resources from cluster #2417
- Kyverno ConfigMap name not consistent in Helm/Docs and install.yaml #2347
- Fixing helm chart documentation inconsistency #2419
- Create/Update policy failing with custom JMESPath #2409
- GenerateRequests are not cleaned up #2332
- NetworkPolicy: from should be an array of objects #2423
- Kyverno misinterprets pod spec environment variable placeholders as references #2413
- CLI | skipped policy message is displayed even if variable is passed #2445
- Update minio to address vulnerabilities #1953
- No warning about background mode when using `any` / `all` in `match` or `exclude` blocks #2300
- Flaky unit test #2406
- Generating a Kyverno Policy throws error "Policy is unstructured" #2155
- Network policy is not getting generated on creation of a pod #2095
- Namespace generate policy fails with `request.operation` precondition #2226
- Fix `any`/`all` matching logic in the background controller #2386
- Run code-generator for 1.5 schema changes #2465
- Generate policies with no Namespace field #2333
- Excluding clusterRoles does not work if nested under any or all #2301
- Fix auto-gen for `validate.foreach` #2464
- "Auto-gen rules for pod controllers" fails when matching kind is "v1/Pod" #2415
- Set Namespace environment variable for initContainer #2499


### Others
- Cannot add label to nodes #2397
- Purge grafana dashboard json from this project #2399


Thanks to all our contributors! 😊

## v1.4.3

## v1.4.3-rc2

### Bug Fixes
- Fix any/all conversion during policy mutation (#2392)
- Fix upgrade issue from 1.4.2 to latest (#2384)

## v1.4.3-rc1

### Enhancements 
- CLI variables should be coming from the resources itself (#1996)
- Adding `ownerRef` with namespace for Kyverno managed webhook configurations (#2263)
- Support new policy report CRD #1753, (#2376)
- Clean up formatting in mutate test file (#2338)
- Add test case for non zero index patches with patchesJson6902 (#2339)
- Cleanup Kustomization configurations (#2274)
- Kyverno CLI `apply` command improvements (#2342, #2331, #2318, #2310, #2296, #2290, #2122, #2120, #2367)
- Validate `path` element begins with a forward slash in `patchesJson6902` (#2117)
- Support gvk in CLI for policies applied on cluster (#2363)
- Update cosign (#2266)
- Allow users to skip policy validation when mutating resources (#2185)
- Allow NetworkPolicy customization (#2287)
- Patch labels to Helm templates (#2262)
- Support for configurable automatic refresh of metrics and selective exposure of metrics at namespace-level (#2268)
- Support global anchor behavior in validation and mutation rules (#2201)



### Bug Fixes
- Unable to use `GreaterThan` operator with `precondition` (#2211)
- Fix `precondition` logic for mutating policies (#2271, #2228, #2352)
- Fix Kyverno Deployment updateStrategy (#1982)
- Helm chart releases are not gated behind something like a tag (#2264)
- Add validation for generate loops (#1941)
- Policy doesn't work when `match.resources.kinds` is set to `Policy/ClusterPolicy` (#2149)
- Kyverno CLI panics when context is added to rule, but not actually used (#2289)
- Generate policies with `background:false` and `synchronize:false` are still re-evaluated every 15mins (#2181)
- Tests applied on excluded resources should succeed (#2295)
- Kyverno CLI with context variables needs documentation (#2291)
- Kyverno CLI test requires var resolution for non-applicable resources (#2331)
- Test command result showing `Notfound` in result (#2296)
- `any/all` in match block fails in the CLI (#2350)
- JMESPath `contains` function behavior not consistent in Kyverno vs upstream (#2345)
- `patchStrategicMerge` fails to mutate if policy written with initContainers object (#1916)
- Check Any and All ResourceFilters during policy mutation (#2373)
- Support variable replacement in the key of annotations (#2316)
- Background scan doesn't work with any/all (#2299)


### Others
- Kyverno gives error when installed with KEDA (#2267)
- Using Argo to deploy, baseline policies are constantly out-of-sync (#2234)
- Policy update, flux2-multi-tenancy fails to update kyverno to v1.4.2-rc3 (#2241)
- Throws a variable substitution error in spite of no variable present in the policy (#2374)

## v1.4.2

### Enhancements 
- Remove unused variable from Kyverno CLI (#2252)

## v1.4.2-rc4

### Enhancements
- Update cosign to v1.0.0 (#2221)
- Helm Chart - Add Network Policy Support (#2210)
- Add platform to bug template (#2246)
- Update Grafana dashboard json with respect to new set of metrics (#2244)
- Automate CLI binaries releases (#2236)
- Removing OwnerReference for webhook configurations (#2251)

### Bug Fixes
- Resolve variables from the resource passed in CLI (#2222)
- Fix CLI panics when variables are passed using set flag (#2224)

## v1.4.2-rc3
