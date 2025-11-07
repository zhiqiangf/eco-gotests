# Pre-Test Cleanup Feature - Quick Summary

## What Was Added

Automatic cleanup of leftover resources **at the beginning of the test suite** before any tests run.

## Why It's Important

When you stop tests with `Ctrl+C` or they fail mid-way, leftover resources remain:
- Test namespaces still exist
- SR-IOV networks still exist  
- VF resources still marked as allocated on nodes
- **Next test run fails** with "Insufficient resources"

## The Solution

New `cleanupLeftoverResources()` function automatically:

1. ✅ **Finds all leftover `e2e-*` namespaces** from previous runs
2. ✅ **Deletes them gracefully** (120s timeout, force delete as fallback)
3. ✅ **Finds all test SR-IOV networks** (25959-, 70821-, etc.)
4. ✅ **Deletes them** (operator cleans up associated NADs)
5. ✅ **Frees up VF resources** on worker nodes
6. ✅ **Logs everything** for troubleshooting

## How It Works

### Execution Flow

```
Test Run Starts
    │
    ▼
BeforeSuite Hook
    │
    ├─► cleanupLeftoverResources() ← NEW!
    │   ├─ "Cleaning up leftover test namespaces..."
    │   ├─ Finds: e2e-25959-cx7anl244
    │   ├─ Finds: e2e-70821-cx7anl244
    │   ├─ Deletes each (with 120s timeout)
    │   │
    │   ├─ "Cleaning up leftover SR-IOV networks..."
    │   ├─ Finds: 25959-cx7anl244
    │   ├─ Finds: 70821-cx7anl244
    │   └─ Deletes each
    │
    ├─ Create fresh test namespace
    ├─ Verify SRIOV operator
    ├─ Pull test images
    │
    ▼
All Tests Run (Clean state!)
```

## Code Changes

### File: `tests/sriov/helpers.go`
- Added `cleanupLeftoverResources()` function (lines 1310-1367)
- Added `namespace` import

### File: `tests/sriov/sriov_basic_test.go`
- Call cleanup in `BeforeSuite` hook (line 113) - **first action**

## Benefits

| Before | After |
|--------|-------|
| ❌ Stop test with Ctrl+C | ✅ Stop test with Ctrl+C |
| ❌ Manual cleanup needed | ✅ Auto cleanup next run |
| ❌ "Insufficient resources" error | ✅ Clean VF resources |
| ❌ Stuck namespaces pile up | ✅ All cleaned automatically |
| ❌ Need to find & delete resources | ✅ Handled transparently |

## Example: Before vs After

### Before This Fix
```
$ Ctrl+C (interrupt test during run)

$ go test ./tests/sriov/... (run tests again)
❌ Error: Insufficient openshift.io/cx7anl244 resources
   Pod stuck in Pending

$ Manual fix required:
  oc delete namespace e2e-25959-cx7anl244
  oc delete sriovnetwork 25959-cx7anl244 -n openshift-sriov-network-operator
  (wait 2-3 minutes for cleanup)

$ go test ./tests/sriov/... (try again)
✅ Finally works
```

### After This Fix
```
$ Ctrl+C (interrupt test during run)

$ go test ./tests/sriov/... (run tests again)
INFO: Cleaning up leftover resources from previous test runs
INFO: Removing leftover test namespace: e2e-25959-cx7anl244
INFO: Removing leftover SR-IOV network: 25959-cx7anl244
INFO: Cleanup of leftover resources completed

✅ Tests run successfully!
```

## What Gets Cleaned

### Namespaces
- Pattern: `e2e-*` (e.g., `e2e-25959-cx7anl244`)
- Timeout: 120 seconds per namespace
- Fallback: Force delete if normal delete times out

### SR-IOV Networks
- Pattern: Test IDs with `-` (e.g., `25959-cx7anl244`, `70821-e810xxv`)
- Location: `openshift-sriov-network-operator` namespace
- Auto-cleanup: Associated NADs cleaned by operator

### Associated Resources (Auto-cleaned by Operator)
- NetworkAttachmentDefinitions (NADs)
- VF allocations on nodes
- SR-IOV configurations

## No Action Needed!

The cleanup is **fully automatic** - just run your tests as normal:

```bash
export SRIOV_DEVICES="cx7anl244:1021:15b3:ens2f0np0"
GOTOOLCHAIN=auto go test ./tests/sriov/... -v -ginkgo.v -timeout 60m
```

**What happens inside:**
1. ✅ Cleanup of leftover resources (automatic)
2. ✅ Create fresh test namespace
3. ✅ Verify SRIOV operator
4. ✅ Your actual tests
5. ✅ Final cleanup

## Troubleshooting

### Q: Cleanup takes a long time?
A: Expected! 120s timeout × multiple namespaces = 2-3 minutes. This ensures thorough cleanup.

### Q: Tests still fail with "Insufficient resources"?
A: Some resources may be stuck. Check cleanup log output for errors. May need manual cleanup.

### Q: How to manually cleanup if needed?
```bash
# List stuck namespaces
oc get ns | grep "e2e-"

# Force delete
oc delete namespace e2e-25959-cx7anl244 --grace-period=0 --force

# List stuck SR-IOV networks
oc get sriovnetwork -n openshift-sriov-network-operator

# Delete
oc delete sriovnetwork 25959-cx7anl244 -n openshift-sriov-network-operator
```

## Summary

✅ **Automatic cleanup before tests run**  
✅ **Prevents resource exhaustion errors**  
✅ **No manual intervention needed**  
✅ **Comprehensive logging for debugging**  
✅ **Graceful error handling with fallbacks**

**Just run your tests - we handle the cleanup!** 🚀

