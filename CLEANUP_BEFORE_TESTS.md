# Pre-Test Cleanup Feature

## Overview

A comprehensive cleanup function has been added to the SRIOV test suite that automatically cleans up any leftover resources from previous failed test runs before starting new tests.

## Problem Solved

When tests are interrupted (e.g., by pressing `Ctrl+C`), they leave behind:
- **Test namespaces** (e.g., `e2e-25959-cx7anl244`, `e2e-70821-cx7anl244`)
- **SR-IOV networks** (e.g., `25959-cx7anl244`, `70821-cx7anl244`)
- **NetworkAttachmentDefinitions (NADs)** in test namespaces
- **Test pods** running in those namespaces
- **Resource allocations** on nodes (VF resources still marked as allocated)

These leftover resources cause subsequent test runs to fail with:
- `Insufficient openshift.io/<device> resources` errors
- Pods stuck in `Pending` state
- `context deadline exceeded` during cleanup

## Solution

The `cleanupLeftoverResources()` function is called at the beginning of the test suite (in `BeforeSuite`) and:

1. **Finds and removes all leftover test namespaces**
   - Pattern: Namespaces starting with `e2e-`
   - Timeout: 120 seconds per namespace
   - Graceful fallback: If normal delete times out, attempts force delete

2. **Finds and removes all leftover SR-IOV networks**
   - Pattern: Networks with test case IDs (e.g., `25959-`, `70821-`, etc.)
   - Automatically cleaned up in the operator namespace
   - Associated NADs are cleaned up by the operator

3. **Logs all cleanup actions**
   - Tracks which resources were found and deleted
   - Reports any errors encountered during cleanup
   - Continues cleanup even if individual deletions fail

## Implementation Details

### Function Signature

```go
func cleanupLeftoverResources(apiClient *clients.Settings, sriovOperatorNamespace string)
```

### Cleanup Sequence

```
BeforeSuite Hook
    │
    ├─ cleanupLeftoverResources() ← NEW!
    │   ├─ List all namespaces
    │   ├─ Find namespaces with "e2e-" prefix
    │   ├─ Delete each namespace (120s timeout)
    │   ├─ Force delete if normal delete times out
    │   │
    │   ├─ List all SR-IOV networks
    │   ├─ Find test networks (25959-, 70821-, etc.)
    │   └─ Delete each network
    │
    ├─ Create test namespace
    ├─ Verify SRIOV operator is deployed
    └─ Pull test images
```

### Files Modified

1. **`helpers.go`** (lines 1310-1367)
   - Added `cleanupLeftoverResources()` function
   - Added `namespace` import

2. **`sriov_basic_test.go`** (lines 111-131)
   - Call `cleanupLeftoverResources()` in `BeforeSuite` hook (first action)

## Behavior

### Scenario 1: Clean Test Start
```
No leftover resources found
├─ Logs: "Cleaning up leftover test namespaces from previous runs"
├─ Logs: "Cleaning up leftover SR-IOV networks from previous runs"
└─ Tests proceed normally
```

### Scenario 2: Previous Test Interrupted (Ctrl+C)
```
Leftover resources found:
├─ e2e-25959-cx7anl244       ← Found and deleted (120s)
├─ e2e-70821-cx7anl244       ← Found and deleted (120s)
├─ 25959-cx7anl244 network   ← Found and deleted
├─ 70821-cx7anl244 network   ← Found and deleted
│
Result: VF resources freed, tests can proceed
```

### Scenario 3: Failed Cleanup During Previous Run
```
Some resources failed to delete:
├─ e2e-25959-cx7anl244       ← Tried delete, failed
├─ Attempted force delete    ← Also failed (namespace stuck)
│
Logs show the error, but cleanup continues
Tests proceed with whatever cleanup succeeded
(Stuck namespaces may need manual intervention)
```

## Example Log Output

```
STEP: Cleaning up leftover resources from previous test runs
STEP: Cleaning up leftover test namespaces from previous runs
"level"=0 "msg"="Removing leftover test namespace" "namespace"="e2e-25959-cx7anl244"
"level"=0 "msg"="Removing leftover test namespace" "namespace"="e2e-70821-cx7anl244"
STEP: Cleaning up leftover SR-IOV networks from previous runs
"level"=0 "msg"="Removing leftover SR-IOV network" "network"="25959-cx7anl244"
"level"=0 "msg"="Removing leftover SR-IOV network" "network"="70821-cx7anl244"
"level"=0 "msg"="Cleanup of leftover resources completed"
```

## Benefits

✅ **Prevents Resource Exhaustion**
- Frees up SR-IOV VF resources after interrupted tests
- Prevents "Insufficient resources" errors in new test runs

✅ **Improves Reliability**
- Ensures clean state before each test run
- Reduces flakiness from leftover state

✅ **Reduces Manual Intervention**
- No need to manually delete namespaces after interrupting tests
- Users can just re-run the tests without cleanup

✅ **Informative Logging**
- Shows exactly what resources are being cleaned up
- Reports errors that occur during cleanup

## Edge Cases Handled

| Scenario | Handling |
|----------|----------|
| Namespace stuck in `Terminating` | Attempts force delete |
| SR-IOV network stuck in deletion | Logs error, continues |
| API client error | Logs error, continues with other resources |
| Multiple failed runs | Cleans up all accumulated namespaces |
| Partial cleanup failure | Proceeds with test anyway, may fail later with better error messages |

## What Gets Cleaned Up

### Namespaces
- All namespaces matching pattern: `e2e-*`
- These are test namespaces created during test runs
- Format: `e2e-<TestID>-<DeviceName>` (e.g., `e2e-25959-cx7anl244`)

### SR-IOV Networks
- Networks in SR-IOV operator namespace
- Matching pattern: Contains `-` and starts with digit `2` or `7`
- Examples: `25959-cx7anl244`, `70821-cx7anl244`, `25963-e810xxv`

### Associated Resources (Cleaned by Operator)
- NetworkAttachmentDefinitions (NADs) in test namespaces
- VF resource allocations on nodes
- SR-IOV policy configurations

## Manual Cleanup (If Needed)

If automatic cleanup fails and resources remain:

```bash
# List leftover namespaces
oc get ns | grep "e2e-"

# Force delete a stuck namespace
oc delete namespace e2e-25959-cx7anl244 --grace-period=0 --force

# List leftover SR-IOV networks
oc get sriovnetwork -n openshift-sriov-network-operator | grep "^[0-9]"

# Delete a stuck SR-IOV network
oc delete sriovnetwork 25959-cx7anl244 -n openshift-sriov-network-operator
```

## Testing the Cleanup

```bash
# Run tests as normal
export SRIOV_DEVICES="cx7anl244:1021:15b3:ens2f0np0"
GOTOOLCHAIN=auto go test ./tests/sriov/... -v -ginkgo.v -timeout 60m

# While tests are running, observe the cleanup in the log output
# Look for: "Cleaning up leftover resources from previous test runs"
```

## Troubleshooting

### Issue: Cleanup takes a long time
**Reason:** Deleting 120+ second timeout × multiple namespaces can take 3-5 minutes
**Solution:** Expected behavior, cleanup is thorough to free all resources

### Issue: Tests still fail with "Insufficient resources"
**Reason:** Cleanup found and deleted namespaces, but some resources are still stuck
**Solution:** Run manual cleanup commands above, then retry tests

### Issue: Cleanup log shows "Failed to delete leftover namespace"
**Reason:** Namespace is stuck in `Terminating` or has finalizers
**Solution:** Manual cleanup may be needed, or wait and retry test run


