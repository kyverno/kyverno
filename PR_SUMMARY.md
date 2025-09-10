# Quick PR Summary

**Branch:** `fix/resourcequota-recreation-preconditions-13687`
**Issue:** #13687 - Kyverno not recreating ResourceQuotas with preconditions

## 🚀 What this PR does

Fixes the bug where generated resources with preconditions aren't properly recreated when deleted. Instead of recreating from cache, we now create UpdateRequests that trigger full policy re-evaluation including context and preconditions.

## 🔧 Key changes

- Modified dynamic watcher to create UpdateRequests on resource deletion
- Added comprehensive tests for the fix
- Maintains backward compatibility

## ✅ Testing

- All existing tests pass ✅
- New unit and integration tests ✅
- Background controller builds successfully ✅

## 📋 Ready for review

The PR is ready for review and includes:
- Detailed description of the problem and solution
- Comprehensive tests
- Backward compatibility
- Clean commit history

**PR URL:** https://github.com/yrsuthari/kyverno/pull/new/fix/resourcequota-recreation-preconditions-13687
