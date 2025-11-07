# Fixes for Test Failures - Test 25961 and 70821

## Issues Identified

### Issue 1: Empty Node Name Error (Test 25961)
**Error Message:**
```
[FAILED] Node name should not be empty
Expected: <string>: 
In [It] at: /root/eco-gotests/tests/sriov/helpers.go:1297
```

**Root Cause:**
The code was trying to access the pod's node name from `clientPod.Definition.Spec.NodeName` immediately after the pod became ready. However, the pod object's Definition field was not refreshed after `WaitUntilReady()` was called, so the node name was not populated in the local object (even though the pod was actually scheduled on a node in the cluster).

**Fix Applied:**
Added a `Pull()` call to refresh the pod definition after waiting for readiness:

```go
// Refresh pod definition to get the latest node name after it was scheduled
err = clientPod.Pull(getAPIClient(), clientPod.Definition.Name, clientPod.Definition.Namespace)
Expect(err).ToNot(HaveOccurred(), "Failed to refresh client pod definition")

clientPodNode := clientPod.Definition.Spec.NodeName
Expect(clientPodNode).NotTo(BeEmpty(), "Client pod node name should not be empty after scheduling")
```

**File:** `tests/sriov/helpers.go` (lines 727-732)

---

### Issue 2: Namespace Deletion Timeout (Test 25961)
**Error Message:**
```
"Failed to delete namespace" "namespace"="e2e-25961-cx7anl244" "error"="context deadline exceeded"
```

**Root Cause:**
The namespace deletion timeout was set to 30 seconds, but SR-IOV resource cleanup (deleting SriovNetwork, NAD, pods with SR-IOV NICs) takes longer than 30 seconds. This caused the namespace deletion to fail with a timeout error.

**Fix Applied:**
Increased namespace deletion timeout from **30 seconds to 120 seconds** to allow sufficient time for SR-IOV resource cleanup.

**Files Changed:**
- `tests/sriov/sriov_basic_test.go` - All 9 tests (lines 206, 264, 321, 379, 442, 502, 559, 653, 715)
- `tests/sriov/helpers.go` - Pod deletion timeout increased from 30 to 60 seconds (lines 771-772)

---

## Summary of Changes

| Component | Old Value | New Value | Impact |
|-----------|-----------|-----------|--------|
| Namespace deletion timeout | 30 seconds | 120 seconds | Allows SR-IOV resource cleanup to complete |
| Pod deletion timeout | 30 seconds | 60 seconds | Ensures pods with SR-IOV NICs are properly cleaned up |
| Pod definition refresh | N/A (missing) | Added Pull() call | Fixes empty node name issue |

---

## Expected Results After Fix

✅ **Test 25961 (SR-IOV VF with auto link state)** should now:
- Successfully refresh pod definition after scheduling
- Retrieve the correct node name for spoof checking verification
- Complete namespace deletion within timeout
- Pass successfully

✅ **Test 70821 (SR-IOV VF with trust enabled)** should now:
- Have VF resources available (after test 25961 cleanup completes)
- Run without resource contention
- Complete successfully

---

## Testing

To verify the fixes work:

```bash
# Run the problematic tests
export SRIOV_DEVICES="cx7anl244:1021:15b3:ens2f0np0"
GOTOOLCHAIN=auto go test ./tests/sriov/... -v -ginkgo.v -timeout 60m \
  -ginkgo.focus "25961|70821"
```

Expected: Both tests should pass without timeout or node name errors.

---

## Root Cause Analysis

### Why These Issues Occurred:

1. **Node Name Issue**: The test framework was caching pod objects and not refreshing them after async operations. The pod WAS scheduled on a node, but the local object wasn't updated.

2. **Timeout Issue**: SR-IOV cleanup is more complex than standard Kubernetes resources:
   - Must delete SriovNetwork (triggers operator cleanup)
   - Must wait for NAD to be deleted from test namespace
   - Must wait for pods to fully terminate (VF cleanup takes time)
   - Must wait for namespace itself to terminate
   
   30 seconds was insufficient for this full cleanup chain.

### Prevention for Future:

- Always refresh objects after async operations (Pull/Refresh)
- Use context-aware timeouts for complex resource cleanup (2x the normal timeout)
- Add diagnostic logging before assertions to catch empty/nil values early


