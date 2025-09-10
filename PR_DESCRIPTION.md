## Description

This PR fixes issue #13687 where Kyverno fails to recreate generated resources when they are deleted and preconditions need to be re-evaluated.

### Problem

When a generated resource (like a ResourceQuota) is deleted, Kyverno's dynamic watcher attempts to recreate it directly from cached data without re-evaluating preconditions or context variables. This causes issues with generation policies that use:

- `apiCall` context variables in preconditions
- Complex conditional logic that depends on the current state of the cluster
- `generateExisting: true` with `synchronize: true`

**Specific scenario from the issue:**
1. A ClusterPolicy generates ResourceQuotas with a precondition checking if "override" quotas exist via apiCall
2. When all ResourceQuotas are deleted from a namespace, the precondition should trigger recreation of the "default" quota
3. However, the old logic just recreated from cache without checking the precondition

### Solution

Modified the dynamic watcher's `handleDelete` method to create **UpdateRequests** instead of directly recreating resources. This triggers the full background generation pipeline which includes:

1. ✅ **Context loading** (including `apiCall` context variables)
2. ✅ **Precondition evaluation** 
3. ✅ **Conditional resource generation** based on current cluster state

### Changes Made

#### Core Fix
- **`pkg/background/gpol/dynamic_watcher.go`**: 
  - Modified `handleDelete` to create UpdateRequests when generated resources are deleted
  - Added helper functions for UpdateRequest creation
  - Enhanced `WatchManager` to accept `kyvernoClient` for UpdateRequest creation
  - Maintains backward compatibility with fallback behavior

#### Supporting Changes
- **`cmd/background-controller/main.go`**: Updated `NewWatchManager` call to pass `kyvernoClient`
- **`pkg/background/gpol/dynamic_watcher_test.go`**: Updated imports for fake client

#### Tests
- **`pkg/background/gpol/dynamic_watcher_deletion_test.go`**: New unit tests verifying:
  - UpdateRequest creation when resources are deleted
  - Fallback behavior when labels are missing
- **`test/conformance/chainsaw/generate/clusterpolicy/standard/existing/resourcequota-recreation-with-preconditions/`**: 
  - End-to-end test reproducing the exact scenario from issue #13687
  - Tests ResourceQuota recreation with apiCall context and preconditions

### Backward Compatibility

✅ **Fully backward compatible**
- Maintains existing behavior for resources without proper labels (fallback)
- No breaking changes to existing APIs
- All existing tests continue to pass

### Testing

- ✅ All existing tests pass
- ✅ New unit tests pass
- ✅ Background controller builds successfully
- ✅ Created comprehensive integration test for the specific issue scenario

## Related Issue

Fixes #13687

## Type of Change

- [x] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)  
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update

## Checklist

- [x] I have read the [contributing guidelines](../CONTRIBUTING.md)
- [x] I have read the [PR documentation](../CREATE_PR.md)
- [x] This is not a duplicate of an existing pull request  
- [x] I have conducted a self-review of my own code
- [x] I have commented my code, particularly in hard-to-understand areas
- [x] I have made corresponding changes to the documentation where applicable
- [x] I have added tests that prove my fix is effective or that my feature works
- [x] New and existing unit tests pass locally with my changes
- [x] Any dependent changes have been merged and published in downstream modules
- [x] I have checked my code and corrected any misspellings
- [x] I have removed commented-out code
- [x] I have included a clear description of the issue and solution