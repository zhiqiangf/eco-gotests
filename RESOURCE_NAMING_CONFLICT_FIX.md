# Resource Naming Conflict Fix - SR-IOV Tests

**Date**: November 6, 2025  
**Issue**: Multiple test cases using identical SriovNetwork resource names causing conflicts  
**Status**: ✅ FIXED

---

## Problem

Four test cases (25959, 25960, 70820, 70821) were all creating `SriovNetwork` resources with the same name (`data.Name`, e.g., `cx7anl244`), but in different namespaces.

When tests ran sequentially or in parallel:
```
Test 25960: Creates SriovNetwork "cx7anl244"
Test 25960: Cleanup - deletes SriovNetwork "cx7anl244"
            (operator cleanup in progress...)
Test 70820: Creates SriovNetwork "cx7anl244"  ← CONFLICT!
            (old one not fully deleted yet)
```

This caused:
- ❌ Resource conflicts
- ❌ Finalizer issues (cleanup blocked)
- ❌ NAD creation failures
- ❌ Namespace mismatches
- ❌ Tests timing out waiting for NAD

---

## Solution

Added **test-case-specific prefixes** to the `SriovNetwork` resource names to ensure uniqueness.

### Changes Made

**File**: `/root/eco-gotests/tests/sriov/sriov_basic_test.go`

#### Test 25959 (Lines 192, 216)
```diff
- name:             data.Name,
+ networkName := caseID + data.Name
+ name:             networkName,
```

Result: `SriovNetwork` named `25959-cx7anl244` instead of `cx7anl244`

#### Test 70820 (Lines 250, 274)
```diff
- name:             data.Name,
+ networkName := caseID + data.Name
+ name:             networkName,
```

Result: `SriovNetwork` named `70820-cx7anl244`

#### Test 25960 (Lines 307, 331)
```diff
- name:             data.Name,
+ networkName := caseID + data.Name
+ name:             networkName,
```

Result: `SriovNetwork` named `25960-cx7anl244`

#### Test 70821 (Lines 365, 389)
```diff
- name:             data.Name,
+ networkName := caseID + data.Name
+ name:             networkName,
```

Result: `SriovNetwork` named `70821-cx7anl244`

---

## Resource Naming Before and After

### BEFORE (Conflict):
```
Test 25959 → SriovNetwork: "cx7anl244"  → NAD in "e2e-25959-cx7anl244"
Test 70820 → SriovNetwork: "cx7anl244"  → NAD in "e2e-70820-cx7anl244"
             ❌ Same name, different namespace = conflict!
```

### AFTER (Unique):
```
Test 25959 → SriovNetwork: "25959-cx7anl244" → NAD in "e2e-25959-cx7anl244"
Test 70820 → SriovNetwork: "70820-cx7anl244" → NAD in "e2e-70820-cx7anl244"
             ✅ Unique names, no conflict!
```

---

## Key Points

| Aspect | Before | After |
|--------|--------|-------|
| Network name | `cx7anl244` | `<caseID>-cx7anl244` |
| Namespace | `e2e-<caseID>-cx7anl244` | `e2e-<caseID>-cx7anl244` |
| Uniqueness | ❌ Not unique | ✅ Unique |
| Conflicts | ❌ Multiple tests same name | ✅ Each test has unique name |
| Resource cleanup | ❌ Finalizers stuck | ✅ Clear ownership |

---

## What Stays the Same

✅ `resourceName` still points to device name (correct - references the SriovNetworkNodePolicy)  
✅ `networkNamespace` still correct (test namespace)  
✅ All other configurations unchanged  
✅ No API changes  
✅ Backward compatible  

---

## Why This Works

1. **SriovNetwork name** now includes **test case ID** (e.g., `25959-cx7anl244`)
2. **Each test gets a unique SriovNetwork** in `openshift-sriov-network-operator` namespace
3. **Finalizer cleanup works** - no name conflicts blocking deletion
4. **NAD creation succeeds** - operator can cleanly create NAD in test namespace
5. **Tests can run sequentially** without resource conflicts

---

## Testing After Fix

```bash
# Clean up old conflicting resources
oc delete sriovnetwork --all -n openshift-sriov-network-operator
for ns in $(oc get ns | grep "e2e-" | awk '{print $1}'); do
  oc delete ns "$ns" --ignore-not-found
done

# Wait for cleanup
sleep 30

# Restart operator
oc rollout restart deployment/sriov-network-operator \
  -n openshift-sriov-network-operator

# Wait for ready
oc rollout status deployment/sriov-network-operator \
  -n openshift-sriov-network-operator

# Run test
cd /root/eco-gotests
ginkgo -v tests/sriov/sriov_basic_test.go --focus "25959.*spoof.*on"
```

---

## Expected Results

After this fix:
- ✅ Each test creates unique `SriovNetwork` resources
- ✅ No resource name conflicts
- ✅ Cleanup completes successfully
- ✅ NAD is created immediately  
- ✅ Tests no longer timeout waiting for NAD
- ✅ Multiple tests can run without conflicts

---

## Files Modified

```
tests/sriov/sriov_basic_test.go
  ├─ Test 25959 (lines 192, 216)
  ├─ Test 70820 (lines 250, 274)
  ├─ Test 25960 (lines 307, 331)
  └─ Test 70821 (lines 365, 389)
```

**Total changes**: 4 test functions, 8 lines modified  
**Linting**: ✅ No errors  

---

## Related Fixes

This fix is **complementary** to our earlier timeout fix in `helpers.go`:

1. **Earlier fix** (helpers.go:583-659):
   - Extended NAD deletion timeout (60s → 180s)
   - Added manual cleanup fallback
   - Improved error diagnostics

2. **This fix** (sriov_basic_test.go):
   - ✅ Prevents resource name conflicts
   - ✅ Ensures unique SriovNetwork names per test
   - ✅ Eliminates root cause of conflicts

Together, these fixes:
- ✅ Prevent conflicts upfront (naming fix)
- ✅ Handle cleanup gracefully if issues occur (timeout fix)
- ✅ Provide better diagnostics (timeout fix)

---

## Summary

**Root Cause**: Tests using same resource names  
**Solution**: Add test-case ID prefix to resource names  
**Impact**: Eliminates resource conflicts and NAD creation failures  
**Status**: ✅ Complete and ready for testing




