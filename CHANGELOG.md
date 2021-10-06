## v1.5.0-rc1
### Note
- With the change of dynamic webhooks, the readiness of the policy is reflected by `.status.ready`, When ready, it means the policy is ready to serve the admission requests.

### Deprecation
- To add a consistent style in flag names the following flags have been deprecated `webhooktimeout`, `gen-workers`,`disable-metrics`, `background-scan`, `auto-update-webhooks` these will be removed in 1.6.0. The new flags are `webhookTimeout`, `genWorkers`, `disablMetrics`, `backgroundScan`, `autoUpdateWebhooks`.

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
